package rackhd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"strings"

	apiclientRedfish "github.com/emccode/gorackhd-redfish/client"
	"github.com/emccode/gorackhd-redfish/client/redfish_v1"
	modelsRedfish "github.com/emccode/gorackhd-redfish/models"
	apiclientMonorail "github.com/emccode/gorackhd/client"
	"github.com/emccode/gorackhd/client/lookups"
	"github.com/emccode/gorackhd/client/nodes"

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
	SSHUser        string
	SSHPassword    string
	SSHPort        int
	SSHKey         string
	Transport      string
	clientMonorail *apiclientMonorail.Monorail
	clientRedfish  *apiclientRedfish.Redfish
}

const (
	defaultEndpoint    = "localhost:8080"
	defaultTransport   = "http"
	defaultSSHUser     = "root"
	defaultSSHPassword = "root"
	defaultSSHPort     = 22
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
			Usage:  "REQUIRED: Specify Node ID, MAC Address or IP Address",
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
			Value:  defaultSSHUser,
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
			Value:  defaultSSHPort,
		},
		/*
			TODO: Grab SSH User and PW from Workflow.
			mcnflag.StringFlag{
				EnvVar: "RACKHD_WORKFLOW_ID",
				Name:   "rackhd-workflow-id",
				Usage:  "workflow ID used to extract SSH user information (optional)",
			},
			TODO: Implicit creation from a pool
			mcnflag.StringFlag{
				EnvVar: "RACKHD_POOL_ID",
				Name:   "rackhd-POOL-id",
				Usage:  "POOL ID",
			},
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
			SSHUser:     defaultSSHUser,
			SSHPort:     defaultSSHPort,
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

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	d.Endpoint = flags.String("rackhd-endpoint")

	d.NodeID = flags.String("rackhd-node-id")
	if d.NodeID == "" {
		return fmt.Errorf("rackhd driver requires the --rackhd-node-id option")
	}

	d.SSHUser = flags.String("rackhd-ssh-user")
	d.SSHPassword = flags.String("rackhd-ssh-password")
	d.SSHPort = flags.Int("rackhd-ssh-port")
	if d.SSHPort == 443 {
		d.Transport = "https"
	} else {
		d.Transport = flags.String("rackhd-transport")
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

	_, err2 := clientRedfish.RedfishV1.GetRoles(nil)
	if err2 != nil {
		return fmt.Errorf("The Redfish API Endpoint is not accessible. Error: %s", err2)
	}

	log.Infof("Test Passed. %v Monorail and Redfish API's are accessible and installation will begin", d.Endpoint)
	return nil
}

func (d *Driver) Create() error {
	//Generate the client
	client := d.getClientMonorail()

	// do a lookup on the ID to retrieve IP information
	resp, err := client.Lookups.GetLookups(&lookups.GetLookupsParams{Q: d.NodeID}, nil)
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
		ipPort := ipAddy + ":" + strconv.Itoa(d.SSHPort)
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

	//create public SSH key
	log.Infof("Creating SSH key...")
	key, err := d.createSSHKey()
	if err != nil {
		return err
	}
	d.SSHKey = strings.TrimSpace(key)

	//TAKEN FROM THE FUSION DRIVER TO USE SSH [THANKS!]
	log.Infof("Copy public SSH key to %s [%s]", d.MachineName, d.IPAddress)
	// create .ssh folder in users home
	if err := executeSSHCommand(fmt.Sprintf("mkdir -p /home/%s/.ssh", d.SSHUser), d); err != nil {
		return err
	}
	// add public ssh key to authorized_keys
	if err := executeSSHCommand(fmt.Sprintf("echo '%v' > /home/%s/.ssh/authorized_keys", d.SSHKey, d.SSHUser), d); err != nil {
		return err
	}
	// make it secure
	if err := executeSSHCommand(fmt.Sprintf("chmod 700 /home/%s/.ssh", d.SSHUser), d); err != nil {
		return err
	}
	// make it secure
	if err := executeSSHCommand(fmt.Sprintf("chmod 600 /home/%s/.ssh/authorized_keys", d.SSHUser), d); err != nil {
		return err
	}

	return nil
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

func (d *Driver) GetSSHUsername() string {
	if d.SSHUser == "" {
		d.SSHUser = "root"
	}
	return d.SSHUser
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

	//If there is no obm (such as Vagrant), send back as Running
	switch respObm.Payload.([]interface{})[0].(map[string]interface{})["service"] {
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

func (d *Driver) Start() error {

	//Get the Out of Band Management Type
	clientMonorail := d.getClientMonorail()
	respObm, errObm := clientMonorail.Nodes.GetNodesIdentifierObm(&nodes.GetNodesIdentifierObmParams{Identifier: d.NodeID}, nil)
	if errObm != nil {
		return errObm
	}

	//If there is no obm (such as Vagrant), nil
	switch respObm.Payload.([]interface{})[0].(map[string]interface{})["service"] {
	case "noop-obm-service":
		return fmt.Errorf("OBM %#v Type Not Supported For Starting: %#v", "noop-obm-service", d.NodeID)
	default:
		log.Debugf("Attempting Turn On: %#v", d.NodeID)
		action := &modelsRedfish.RackHDResetAction{
			ResetType: "On",
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

func (d *Driver) Stop() error {
	//Get the Out of Band Management Type
	clientMonorail := d.getClientMonorail()
	respObm, errObm := clientMonorail.Nodes.GetNodesIdentifierObm(&nodes.GetNodesIdentifierObmParams{Identifier: d.NodeID}, nil)
	if errObm != nil {
		return errObm
	}

	//If there is no obm (such as Vagrant), nil
	switch respObm.Payload.([]interface{})[0].(map[string]interface{})["service"] {
	case "noop-obm-service":
		return fmt.Errorf("OBM %#v Type Not Supported For Stopping: %#v", "noop-obm-service", d.NodeID)
	default:
		log.Debugf("Attempting Graceful Shutdown of: %#v", d.NodeID)
		action := &modelsRedfish.RackHDResetAction{
			ResetType: "GracefulShutdown",
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

func (d *Driver) Remove() error {
	//Get the Out of Band Management Type
	clientMonorail := d.getClientMonorail()
	respObm, errObm := clientMonorail.Nodes.GetNodesIdentifierObm(&nodes.GetNodesIdentifierObmParams{Identifier: d.NodeID}, nil)
	if errObm != nil {
		return errObm
	}

	//If there is no obm (such as Vagrant), nil
	switch respObm.Payload.([]interface{})[0].(map[string]interface{})["service"] {
	case "noop-obm-service":
		log.Debugf("OBM %#v Type Not Supported For Shutdown: %#v", "noop-obm-service", d.NodeID)
	default:
		log.Debugf("Attempting Graceful Shutdown of: %#v", d.NodeID)
		action := &modelsRedfish.RackHDResetAction{
			ResetType: "GracefulShutdown",
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

	//If there is no obm (such as Vagrant), nil
	switch respObm.Payload.([]interface{})[0].(map[string]interface{})["service"] {
	case "noop-obm-service":
		return fmt.Errorf("OBM Type Not Supported: %#v, %#v", "noop-obm-service", d.NodeID)
	default:
		log.Debugf("Attempting Restart of: %#v", d.NodeID)
		action := &modelsRedfish.RackHDResetAction{
			ResetType: "GracefulRestart",
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

func (d *Driver) Kill() error {
	//Get the Out of Band Management Type
	clientMonorail := d.getClientMonorail()
	respObm, errObm := clientMonorail.Nodes.GetNodesIdentifierObm(&nodes.GetNodesIdentifierObmParams{Identifier: d.NodeID}, nil)
	if errObm != nil {
		return errObm
	}

	//If there is no obm (such as Vagrant), nil
	switch respObm.Payload.([]interface{})[0].(map[string]interface{})["service"] {
	case "noop-obm-service":
		return fmt.Errorf("OBM Type Not Supported: %#v, %#v", "noop-obm-service", d.NodeID)
	default:
		log.Debugf("Attempting Force Off of: %#v", d.NodeID)
		action := &modelsRedfish.RackHDResetAction{
			ResetType: "ForceOff",
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
		User: d.SSHUser,
		Auth: []cryptossh.AuthMethod{
			cryptossh.Password(d.SSHPassword),
		},
	}

	client, err := cryptossh.Dial("tcp", fmt.Sprintf("%s:%d", d.IPAddress, d.SSHPort), config)
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
