package rackhd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	apiclientRedfish "github.com/emccode/gorackhd-redfish/client"
	"github.com/emccode/gorackhd-redfish/client/redfish_v1"
	modelsRedfish "github.com/emccode/gorackhd-redfish/models"
	apiclientMonorail "github.com/emccode/gorackhd/client"
	"github.com/emccode/gorackhd/client/lookups"
	"github.com/emccode/gorackhd/client/nodes"
	"github.com/emccode/gorackhd/client/skus"
	"github.com/emccode/gorackhd/client/workflow"
	modelsMonorail "github.com/emccode/gorackhd/models"

	// Need the *old* style libraries for redfish
	red_httptransport "github.com/go-swagger/go-swagger/httpkit/client"
	red_strfmt "github.com/go-swagger/go-swagger/strfmt"

	mono_httptransport "github.com/go-openapi/runtime/client"
	mono_strfmt "github.com/go-openapi/strfmt"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnflag"
	"github.com/docker/machine/libmachine/ssh"
	"github.com/docker/machine/libmachine/state"

	cryptossh "golang.org/x/crypto/ssh"
)

type Driver struct {
	*drivers.BaseDriver
	Endpoint       string
	NodeID         string
	SkuID          string
	SkuName        string
	WorkflowName   string
	SSHPassword    string
	Transport      string
	clientMonorail *apiclientMonorail.Monorail
	clientRedfish  *apiclientRedfish.Redfish
}

const (
	defaultEndpoint    = "localhost:8080"
	defaultTransport   = "http"
	defaultSSHPassword = "root"
)

func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		mcnflag.StringFlag{
			EnvVar: "RACKHD_ENDPOINT",
			Name:   "rackhd-endpoint",
			Usage:  "RackHD Endpoint for API traffic",
			Value:  defaultEndpoint,
		},
		mcnflag.StringFlag{
			EnvVar: "RACKHD_NODE_ID",
			Name:   "rackhd-node-id",
			Usage:  "Specify Node ID, MAC Address or IP Address",
		},
		mcnflag.StringFlag{
			EnvVar: "RACKHD_SKU_ID",
			Name:   "rackhd-sku-id",
			Usage:  "SKU ID to use as pool of nodes to choose from",
		},
		mcnflag.StringFlag{
			EnvVar: "RACKHD_SKU_NAME",
			Name:   "rackhd-sku-name",
			Usage:  "Friendly SKU NAME to use as pool of nodes to choose from",
		},
		mcnflag.StringFlag{
			EnvVar: "RACKHD_WORKFLOW_NAME",
			Name:   "rackhd-workflow-name",
			Usage:  "Name of workflow to invoke after node is chosen (optional)",
		},
		mcnflag.StringFlag{
			EnvVar: "RACKHD_TRANSPORT",
			Name:   "rackhd-transport",
			Usage:  "RackHD Endpoint Transport. Specify http or https. HTTP is default",
			Value:  defaultTransport,
		},
		mcnflag.StringFlag{
			EnvVar: "RACKHD_SSH_USER",
			Name:   "rackhd-ssh-user",
			Usage:  "ssh user (default:root)",
		},
		mcnflag.StringFlag{
			EnvVar: "RACKHD_SSH_PASSWORD",
			Name:   "rackhd-ssh-password",
			Usage:  "ssh password (default:root)",
			Value:  defaultSSHPassword,
		},
		mcnflag.IntFlag{
			EnvVar: "RACKHD_SSH_PORT",
			Name:   "rackhd-ssh-port",
			Usage:  "ssh port (default:22)",
		},
		mcnflag.StringFlag{
			EnvVar: "RACKHD_SSH_KEY",
			Name:   "rackhd-ssh-key",
			Usage:  "SSH private key path (if not provided, default SSH key will be used)",
		},
		/*
			TODO: API Authentication Values. Will be detemined for v 2.0 of API
			mcnflag.StringFlag{
				EnvVar: "RACKHD_ENDPOINT_AUTH",
				Name:   "rackhd-ENDPOINT_AUTH,
				Usage:  "ENDPOINT_AUTH",
			},
		*/
	}
}

func NewDriver(hostName, storePath string) *Driver {
	return &Driver{
		Endpoint:    defaultEndpoint,
		SSHPassword: defaultSSHPassword,
		Transport:   defaultTransport,
		BaseDriver: &drivers.BaseDriver{
			MachineName: hostName,
			StorePath:   storePath,
		},
	}
}

func (d *Driver) GetMachineName() string {
	return d.MachineName
}

func (d *Driver) DriverName() string {
	return "rackhd"
}

/* Need this silly wrapper around d.GetSSHPort() because
   BaseDriver.GetSSHPort() returns an (int, error), but err
   is always hardcoded to nil. Want to use it to return the
   port because it handles defaults already, but can't use it
   "inline" because it's a multi-return Value
*/
func (d *Driver) getSSHPort() int {
	port, _ := d.GetSSHPort()
	return port
}

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	d.Endpoint = flags.String("rackhd-endpoint")

	d.NodeID = flags.String("rackhd-node-id")
	d.SkuID = flags.String("rackhd-sku-id")
	d.SkuName = flags.String("rackhd-sku-name")
	if d.NodeID == "" && d.SkuID == "" && d.SkuName == "" {
		return fmt.Errorf("rackhd driver requires either the --rackhd-node-id or --rackhd-sku-[id/name] option")
	}
	if d.NodeID != "" && (d.SkuID != "" || d.SkuName != "") {
		return fmt.Errorf("rackhd driver accepts either the --rackhd-node-id or --rackhd-sku-[id/name] option, not both")
	}
	if d.SkuID != "" && d.SkuName != "" {
		return fmt.Errorf("rackhd driver accepts either the --rackhd-sku-id or --rackhd-sku-name option, not both")
	}

	d.WorkflowName = flags.String("rackhd-workflow-name")

	d.SSHUser = flags.String("rackhd-ssh-user")
	d.SSHPassword = flags.String("rackhd-ssh-password")
	d.SSHPort = flags.Int("rackhd-ssh-port")
	if d.SSHPort == 443 {
		d.Transport = "https"
	} else {
		d.Transport = flags.String("rackhd-transport")
	}

	d.SSHKeyPath = flags.String("rackhd-ssh-key")
	if d.SSHKeyPath != "" {
		if _, err := os.Stat(d.SSHKeyPath); os.IsNotExist(err) {
			return fmt.Errorf("SSH key does not exist: %q", d.SSHKeyPath)
		}
	}

	return nil
}

func (d *Driver) PreCreateCheck() error {
	log.Infof("Testing accessibility of endpoint: %v", d.Endpoint)
	//Generate the client
	clientMonorail := d.getClientMonorail()
	//do a test to see if the server is available. 2nd Nil is authentication params
	_, err := clientMonorail.Config.GetConfig(nil, nil)
	if err != nil {
		return fmt.Errorf("The Monorail API Endpoint is not accessible. Error: %s", err)
	}

	clientRedfish := d.getClientRedfish()

	_, err = clientRedfish.RedfishV1.ListRoles(nil)
	if err != nil {
		return fmt.Errorf("The Redfish API Endpoint is not accessible. Error: %s", err)
	}

	log.Infof("Test Passed. %v Monorail and Redfish API's are accessible and installation will begin", d.Endpoint)

	if d.SkuName != "" {
		log.Debugf("Looking up SKU ID by name")
		err = d.lookupSkuByName(clientMonorail)
		if err != nil {
			return err
		}
	}

	if d.SkuID != "" {
		log.Infof("Looking for available node within SKU")
		err = d.chooseNode(clientMonorail)
		if err != nil {
			return err
		}
		log.Infof("Found a free node within SKU, Node ID: %v", d.NodeID)
	}

	if d.SSHKeyPath == "" {
		log.Infof("No SSH Key specified. Will attempt login with user/pass and upload generated key pair")
	}

	return nil
}

func (d *Driver) Create() error {
	//Generate the client
	client := d.getClientMonorail()

	if d.WorkflowName != "" {
		wfInstance, err := d.applyWorkflow(client)
		if err != nil {
			return err
		}
		log.Debugf("Workflow %s applied as instance id %s", d.WorkflowName, wfInstance)
		err = d.waitForWorkflow(client, wfInstance, 60, 15)
		if err != nil {
			return err
		}
	}

	return d.checkConnectivity(client)
}

func (d *Driver) chooseNode(client *apiclientMonorail.Monorail) error {
	skuParams := skus.GetSkusIdentifierNodesParams{}
	skuParams.WithIdentifier(d.SkuID)
	resp, err := client.Skus.GetSkusIdentifierNodes(&skuParams, nil)
	if err != nil {
		return err
	}

	var chosenNode modelsMonorail.Node
	log.Debugf("%v", resp)
	for _, node := range resp.Payload {
		n := &modelsMonorail.Node{}
		buf, err := json.Marshal(node)
		if err != nil {
			return err
		}
		err = json.Unmarshal(buf, n)
		if err != nil {
			return err
		}
		tags := getTags(&n.Tags)
		if !stringInSlice("dockermachine", tags) {
			chosenNode = *n
			break
		}
	}
	if chosenNode.ID == "" {
		return fmt.Errorf("No suitable node found in SKU")
	}

	d.NodeID = chosenNode.ID

	err = d.tagNode(d.NodeID, "dockermachine")
	if err != nil {
		return err
	}
	return nil
}

func (d *Driver) applyWorkflow(client *apiclientMonorail.Monorail) (string, error) {
	// POST workflow to node
	params := nodes.NewPostNodesIdentifierWorkflowsParams()
	params.WithIdentifier(d.NodeID)
	params.WithName(d.WorkflowName)
	resp, err := client.Nodes.PostNodesIdentifierWorkflows(params, nil)
	if err != nil {
		return "", err
	}

	var id interface{}
	id, err = getRootLevelVal(resp.Payload.(map[string]interface{}), "instanceId")
	if err != nil {
		return "", err
	}

	return id.(string), nil
}

func (d *Driver) waitForWorkflow(client *apiclientMonorail.Monorail, wfInstance string, timeoutMins, pollSecs int) error {
	timeout := time.After(time.Duration(timeoutMins) * time.Minute)
	tick := time.Tick(time.Duration(pollSecs) * time.Second)
	log.Debugf("Waiting up to %v minutes for workflow to complete", timeoutMins)
	log.Debugf("checking status every %v seconds", pollSecs)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("Timeout waiting for workflow to finish")
		case <-tick:
			// Check if workflow is finished or still running
			params := workflow.NewGetWorkflowsInstanceIDParams()
			params.WithInstanceID(wfInstance)
			resp, err := client.Workflow.GetWorkflowsInstanceID(params, nil)
			if err != nil {
				return err
			}

			var status interface{}
			status, err = getRootLevelVal(resp.Payload.(map[string]interface{}), "_status")
			if err != nil {
				return err
			}
			if status.(string) == "succeeded" {
				log.Debugf("Worklow successful!")
				return nil
			} else if status.(string) != "running" {
				return fmt.Errorf("Workflow appears to have failed")
			}
		}
	}
}

func getRootLevelVal(payload map[string]interface{}, keyToFind string) (interface{}, error) {
	for key, val := range payload {
		if key == keyToFind {
			return val, nil
		}
	}
	return nil, fmt.Errorf("Key %v not found", keyToFind)
}

func (d *Driver) checkConnectivity(client *apiclientMonorail.Monorail) error {

	// do a lookup on the ID to retrieve IP information
	resp, err := client.Lookups.GetLookups(&lookups.GetLookupsParams{Q: &d.NodeID}, nil)
	if err != nil {
		return err
	}

	// new slice for all IP addresses found for the node
	ipAddSlice := make([]string, 0)

	//loop through the response and grab all the IP addresses
	for _, v := range resp.Payload {
		if rec, ok := v.(map[string]interface{}); ok {
			for key, val := range rec {
				if key == "ipAddress" {
					log.Debugf("Found IP Address for Node ID: %v", val.(string))
					ipAddSlice = append(ipAddSlice, val.(string))
				}
			}
		}
	}

	//if the slice is empty that means there are no IPs
	if len(ipAddSlice) <= 0 {
		return fmt.Errorf("No IP addresses are associated with the Node ID specified. Error: %s", err)
	}

	// loop through slice and see if we can connect to the ip:ssh-port
	for _, ipAddy := range ipAddSlice {
		ipPort := ipAddy + ":" + strconv.Itoa(d.getSSHPort())
		log.Debugf("Testing connection to: %v", ipPort)
		conn, err := net.DialTimeout("tcp", ipPort, 25000000000)
		if err != nil {
			log.Debugf("Connection failed on: %v", ipPort)
		} else {
			log.Infof("Connection succeeded on: %v", ipPort)
			d.IPAddress = string(ipAddy)
			conn.Close()
			break
		}
	}

	if d.IPAddress == "" {
		return fmt.Errorf("No IP addresses are accessible on this network to the Node ID specified. Error: %s", err)
	}

	if d.SSHKeyPath == "" {
		//create public SSH key
		log.Infof("Creating SSH key...")
		pubkey, err := d.createSSHKey()
		if err != nil {
			return err
		}
		pubkey = strings.TrimSpace(pubkey)

		//TAKEN FROM THE FUSION DRIVER TO USE SSH [THANKS!]
		log.Infof("Copying public SSH key to %s [%s]", d.MachineName, d.IPAddress)
		// create .ssh folder in users home
		if err := executeSSHCommand(fmt.Sprintf("mkdir -p /home/%s/.ssh", d.GetSSHUsername()), d); err != nil {
			return err
		}
		// add public ssh key to authorized_keys
		if err := executeSSHCommand(fmt.Sprintf("echo '%v' > /home/%s/.ssh/authorized_keys", pubkey, d.GetSSHUsername()), d); err != nil {
			return err
		}
		// make it secure
		if err := executeSSHCommand(fmt.Sprintf("chmod 700 /home/%s/.ssh", d.GetSSHUsername()), d); err != nil {
			return err
		}
		// make it secure
		if err := executeSSHCommand(fmt.Sprintf("chmod 600 /home/%s/.ssh/authorized_keys", d.GetSSHUsername()), d); err != nil {
			return err
		}
	}

	return nil
}

func (d *Driver) lookupSkuByName(client *apiclientMonorail.Monorail) error {
	// Get list of all Skus
	resp, err := client.Skus.GetSkus(nil, nil)
	if err != nil {
		return err
	}

	log.Debugf("%v", resp)
	n := &modelsMonorail.Sku{}
	for _, sku := range resp.Payload {
		buf, err := json.Marshal(sku)
		if err != nil {
			return err
		}
		err = json.Unmarshal(buf, n)
		if err != nil {
			return err
		}

		if n.Name == d.SkuName {
			d.SkuID = n.ID
			return nil
		}
	}
	return fmt.Errorf("No matching SKU found")
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

func (d *Driver) createSSHKey() (string, error) {
	if err := ssh.GenerateSSHKey(d.GetSSHKeyPath()); err != nil {
		return "", err
	}

	publicKey, err := ioutil.ReadFile(d.publicSSHKeyPath())
	if err != nil {
		return "", err
	}

	return string(publicKey), nil
}

func (d *Driver) GetURL() (string, error) {
	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tcp://%s:2376", ip), nil
}

func (d *Driver) GetIP() (string, error) {
	if d.IPAddress == "" {
		return "", fmt.Errorf("IP address is not set")
	}
	return d.IPAddress, nil
}

func (d *Driver) GetState() (state.State, error) {

	//Get the Out of Band Management Type
	clientMonorail := d.getClientMonorail()
	respObm, errObm := clientMonorail.Nodes.GetNodesIdentifierObm(&nodes.GetNodesIdentifierObmParams{Identifier: d.NodeID}, nil)
	if errObm != nil {
		return state.None, errObm
	}

	if len(respObm.Payload) > 0 {
		//If there is no obm (such as Vagrant), send back as Running
		switch respObm.Payload[0].(map[string]interface{})["service"] {
		case "noop-obm-service":
			return state.Running, nil
		default:
			//Generate the client
			clientRedfish := d.getClientRedfish()

			// do a lookup on the Node ID to retrieve Power information
			resp, err := clientRedfish.RedfishV1.GetSystem(&redfish_v1.GetSystemParams{Identifier: d.NodeID})
			if err != nil {
				return state.None, nil
			}
			switch resp.Payload.PowerState {
			case "Online", "online", "Up", "up", "On", "on":
				return state.Running, nil
			case "Offline", "offline", "Down", "down", "Off", "off":
				return state.Stopped, nil
			case "Unknown", "unknown":
				return state.None, nil
			default:
				return state.Running, nil
			}
		}
	}
	return state.None, nil
}

func (d *Driver) Start() error {

	//Get the Out of Band Management Type
	clientMonorail := d.getClientMonorail()
	respObm, errObm := clientMonorail.Nodes.GetNodesIdentifierObm(&nodes.GetNodesIdentifierObmParams{Identifier: d.NodeID}, nil)
	if errObm != nil {
		return errObm
	}

	if len(respObm.Payload) > 0 {
		//If there is no obm (such as Vagrant), nil
		switch respObm.Payload[0].(map[string]interface{})["service"] {
		case "noop-obm-service":
			return fmt.Errorf("OBM %#v Type Not Supported For Starting: %#v", "noop-obm-service", d.NodeID)
		default:
			log.Debugf("Attempting Turn On: %#v", d.NodeID)
			resetType := "On"
			action := &modelsRedfish.RackHDResetActionResetAction{
				ResetType: &resetType,
			}

			clientRedfish := d.getClientRedfish()

			_, err := clientRedfish.RedfishV1.DoReset(&redfish_v1.DoResetParams{Identifier: d.NodeID, Payload: action})
			if err != nil {
				return fmt.Errorf("There was an issue Powering On the Server. Error: %s", err)
			}

			log.Debugf("Node has succussfully been powered on: %#v", d.NodeID)
			return nil
		}
	}
	return fmt.Errorf("There was an issue Powering On the Server. No OBM detected")
}

func (d *Driver) Stop() error {
	//Get the Out of Band Management Type
	clientMonorail := d.getClientMonorail()
	respObm, errObm := clientMonorail.Nodes.GetNodesIdentifierObm(&nodes.GetNodesIdentifierObmParams{Identifier: d.NodeID}, nil)
	if errObm != nil {
		return errObm
	}

	if len(respObm.Payload) > 0 {
		//If there is no obm (such as Vagrant), nil
		switch respObm.Payload[0].(map[string]interface{})["service"] {
		case "noop-obm-service":
			return fmt.Errorf("OBM %#v Type Not Supported For Stopping: %#v", "noop-obm-service", d.NodeID)
		default:
			log.Debugf("Attempting Graceful Shutdown of: %#v", d.NodeID)
			resetType := "GracefulShutdown"
			action := &modelsRedfish.RackHDResetActionResetAction{
				ResetType: &resetType,
			}

			clientRedfish := d.getClientRedfish()

			_, err := clientRedfish.RedfishV1.DoReset(&redfish_v1.DoResetParams{Identifier: d.NodeID, Payload: action})
			if err != nil {
				return fmt.Errorf("There was an issue Shutting Down the Server. Error: %s", err)
			}
			log.Debugf("Node has succussfully been shutdown: %#v", d.NodeID)
			return nil
		}
	}
	return fmt.Errorf("There was an issue Shutting Down the Server. Error: No OBM detected")
}

func (d *Driver) Remove() error {
	//Get the Out of Band Management Type
	clientMonorail := d.getClientMonorail()
	respObm, errObm := clientMonorail.Nodes.GetNodesIdentifierObm(&nodes.GetNodesIdentifierObmParams{Identifier: d.NodeID}, nil)
	if errObm != nil {
		return errObm
	}

	if len(respObm.Payload) > 0 {
		//If there is no obm (such as Vagrant), nil
		switch respObm.Payload[0].(map[string]interface{})["service"] {
		case "noop-obm-service":
			log.Debugf("OBM %#v Type Not Supported For Shutdown: %#v", "noop-obm-service", d.NodeID)
		default:
			log.Debugf("Attempting Graceful Shutdown of: %#v", d.NodeID)
			resetType := "GracefulShutdown"
			action := &modelsRedfish.RackHDResetActionResetAction{
				ResetType: &resetType,
			}

			clientRedfish := d.getClientRedfish()

			_, err := clientRedfish.RedfishV1.DoReset(&redfish_v1.DoResetParams{Identifier: d.NodeID, Payload: action})
			if err != nil {
				log.Infof("There was an issue Shutting Down the Server. Error: %s", err)
				//return fmt.Errorf("There was an issue Shutting Down the Server. Error: %s", err)
			} else {
				log.Debugf("Node has succussfully been shutdown: %#v", d.NodeID)
			}
		}
	} else {
		log.Infof("There was an issue Shutting Down the Server. Error: No OBM detected")
	}

	//Remove the Node from RackHD Inventory
	log.Debugf("Removing Node From RackHD: %#v", d.NodeID)
	_, err2 := clientMonorail.Nodes.DeleteNodesIdentifier(&nodes.DeleteNodesIdentifierParams{Identifier: d.NodeID}, nil)
	if err2 != nil {
		return err2
	}
	log.Debugf("Successfully Removed Node From RackHD: %#v", d.NodeID)

	return nil
}

func (d *Driver) Restart() error {
	//Get the Out of Band Management Type
	clientMonorail := d.getClientMonorail()
	respObm, errObm := clientMonorail.Nodes.GetNodesIdentifierObm(&nodes.GetNodesIdentifierObmParams{Identifier: d.NodeID}, nil)
	if errObm != nil {
		return errObm
	}

	if len(respObm.Payload) > 0 {
		//If there is no obm (such as Vagrant), nil
		switch respObm.Payload[0].(map[string]interface{})["service"] {
		case "noop-obm-service":
			return fmt.Errorf("OBM Type Not Supported: %#v, %#v", "noop-obm-service", d.NodeID)
		default:
			log.Debugf("Attempting Restart of: %#v", d.NodeID)
			resetType := "GracefulRestart"
			action := &modelsRedfish.RackHDResetActionResetAction{
				ResetType: &resetType,
			}

			clientRedfish := d.getClientRedfish()

			_, err := clientRedfish.RedfishV1.DoReset(&redfish_v1.DoResetParams{Identifier: d.NodeID, Payload: action})
			if err != nil {
				return fmt.Errorf("There was an issue Shutting Down the Server. Error: %s", err)
			}
			log.Debugf("Successfully restarted node: %#v", d.NodeID)
			return nil
		}
	}
	return fmt.Errorf("There was an issue Shutting Down the Server. Error: No OBM detected")
}

func (d *Driver) Kill() error {
	//Get the Out of Band Management Type
	clientMonorail := d.getClientMonorail()
	respObm, errObm := clientMonorail.Nodes.GetNodesIdentifierObm(&nodes.GetNodesIdentifierObmParams{Identifier: d.NodeID}, nil)
	if errObm != nil {
		return errObm
	}

	if len(respObm.Payload) > 0 {
		//If there is no obm (such as Vagrant), nil
		switch respObm.Payload[0].(map[string]interface{})["service"] {
		case "noop-obm-service":
			return fmt.Errorf("OBM Type Not Supported: %#v, %#v", "noop-obm-service", d.NodeID)
		default:
			log.Debugf("Attempting Force Off of: %#v", d.NodeID)
			resetType := "ForceOff"
			action := &modelsRedfish.RackHDResetActionResetAction{
				ResetType: &resetType,
			}

			clientRedfish := d.getClientRedfish()

			_, err := clientRedfish.RedfishV1.DoReset(&redfish_v1.DoResetParams{Identifier: d.NodeID, Payload: action})
			if err != nil {
				return fmt.Errorf("There was an issue Shutting Down the Server. Error: %s", err)
			}
			log.Debugf("Successfully turned off node: %#v", d.NodeID)
			return nil
		}
	}
	return fmt.Errorf("There was an issue Shutting Down the Server. Error: No OBM detected")
}

func (d *Driver) tagNode(targetNode, targetTag string) error {
	clientMonorail := d.getClientMonorail()
	params := nodes.NewPatchNodesIdentifierTagsParams()
	body := make(map[string]interface{})
	var tags [1]string
	tags[0] = targetTag
	body["tags"] = tags

	params.WithBody(body)
	params.WithIdentifier(targetNode)
	_, err := clientMonorail.Nodes.PatchNodesIdentifierTags(params, nil)
	return err
}

func (d *Driver) getClientMonorail() *apiclientMonorail.Monorail {
	log.Debugf("Getting RackHD Monorail Client")
	if d.clientMonorail == nil {
		// create the transport
		/** Will Need to determine changes for v 2.0 API **/
		transport := mono_httptransport.New(d.Endpoint, "/api/1.1", []string{d.Transport})
		// create the API client, with the transport
		d.clientMonorail = apiclientMonorail.New(transport, mono_strfmt.Default)
	}
	return d.clientMonorail
}

func (d *Driver) getClientRedfish() *apiclientRedfish.Redfish {
	log.Debugf("Getting RackHD Redfish Client")
	if d.clientRedfish == nil {
		// create the transport
		transport := red_httptransport.New(d.Endpoint, "/redfish/v1", []string{d.Transport})
		// create the API client, with the transport
		d.clientRedfish = apiclientRedfish.New(transport, red_strfmt.Default)
	}
	return d.clientRedfish
}

func (d *Driver) publicSSHKeyPath() string {
	return d.GetSSHKeyPath() + ".pub"
}

// execute command over SSH with user / password authentication
func executeSSHCommand(command string, d *Driver) error {
	log.Debugf("Execute executeSSHCommand: %s", command)

	config := &cryptossh.ClientConfig{
		User: d.GetSSHUsername(),
		Auth: []cryptossh.AuthMethod{
			cryptossh.Password(d.SSHPassword),
		},
	}

	client, err := cryptossh.Dial("tcp", fmt.Sprintf("%s:%d", d.IPAddress, d.getSSHPort()), config)
	if err != nil {
		log.Debugf("Failed to dial:", err)
		return err
	}

	session, err := client.NewSession()
	if err != nil {
		log.Debugf("Failed to create session: " + err.Error())
		return err
	}
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b

	if err := session.Run(command); err != nil {
		log.Debugf("Failed to run: " + err.Error())
		return err
	}
	log.Debugf("Stdout from executeSSHCommand: %s", b.String())

	return nil
}

func getTags(input *[]interface{}) []string {
	tags := make([]string, len(*input))
	for i, tag := range *input {
		tags[i] = tag.(string)
	}
	return tags
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
