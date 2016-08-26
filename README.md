# RackHD Machine Driver [![Build Status](https://travis-ci.org/emccode/docker-machine-rackhd.svg?branch=master)](https://travis-ci.org/emccode/docker-machine-rackhd)

Use [Docker Machine](https://github.com/docker/machine) to create Docker hosts with [RackHD](https://github.com/RackHD/RackHD). This is the first solution that enables bare-metal provisioning of [Docker](https://github.com/docker/docker) and configuration of [Docker Swarm](https://github.com/docker/swarm) to a hardware agnostic server infrastructure.

RackHD Machine Driver increases efficiency by using RackHD APIs to drive automation for all compatible hardware. This entirely new solution differs from other available drivers that are focused on automating virtual machines or proprietary stacks, and allows users to capture all the robust benefits such as increased application performance and reduced management complexity by removing the hypervisor layer.

## Installation

**Linux & Mac OSX**:
```
curl -L https://github.com/emccode/docker-machine-rackhd/releases/download/v0.2.0/docker-machine-driver-rackhd.`uname -s`-`uname -m` >/usr/local/bin/docker-machine-driver-rackhd &&  chmod +x /usr/local/bin/docker-machine-driver-rackhd
```

## Using the driver

To use the driver first make sure you are running at least [version
0.5.1 of `docker-machine`](https://github.com/docker/machine/releases).

```
$ docker-machine -v
docker-machine version 0.5.1 (7e8e38e)
```

Verify `docker-machine` can access the RackHD driver by issuing the `--help` or ` | more` flags. A list of RackHD options will be displayed on the menu.

```
$ docker-machine create -d rackhd | more
Usage: docker-machine create [OPTIONS] [arg...]
```

## Create a Machine

Specify `rackhd` as the driver with `--driver` or `-d` create flags then accompany it with any of the following options as additional parameters.

Options:

| Option                  |  Environment Variable | Default | Description                                     |
|-------------------------|:---------------------:|---------|-------------------------------------------------|
| --rackhd-endpoint    | RACKHD_ENDPOINT  |     localhost:8080    | RackHD Endpoint for API traffic           |
| --rackhd-transport   | RACKHD_TRANSPORT  |    http     | RackHD Endpoint Transport. Specify http or https |
| --rackhd-node-id     | RACKHD_NODE_ID |         | Specify Node ID, MAC Address or IP Address           |
| --rackhd-sku-id      | RACKHD_SKU_ID |         | ID of SKU to pick a node from           |
| --rackhd-sku-name    | RACKHD_SKU_NAME |         | Name of SKU to pick a node from           |
| --rackhd-ssh-user    | RACKHD_SSH_USER  |    root    | SSH User Name for the node        |
| --rackhd-ssh-key     | RACKHD_SSH_KEY |       | Path to an existing SSH private key to SSH into node    |
| --rackhd-ssh-password | RACKHD_SSH_PASSWORD   |    root   | SSH Password for the node (only use if no key is present) |
| --rackhd-ssh-port    | RACKHD_SSH_PORT   |    22    | SSH Port for the node          |
| --rackhd-ssh-attempts | RACKHD_SSH_ATTEMPTS |   10    | Number of attempts to check that SSH port is available    |
| --rackhd-ssh-timeout | RACKHD_SSH_TIMEOUT   | 15    | Timeout (in seconds) for checking that SSH port is up  |
| --rackhd-workflow-name | RACKHD_WORKFLOW_NAME |     | Name of RackHD workflow to run on node  |
| --rackhd-workflow-poll | RACKHD_WORKFLOW_POLL |  15 | Frequency in seconds to poll for status of active workflow  |
| --rackhd-workflow-timeout | RACKHD_WORKFLOW_TIMEOUT |  60 | Max time in minutes to wait for workflow to finish  |

**NOTE:** Specifying either a Node ID *or* a SKU is required.

The driver can work by either specifying a Node ID to work against, or the driver can choose a node from an existing SKU (which acts as a pool of nodes). When given a specific Node ID, the Node must be a `compute` instance, not an `enclosure`.

These examples will function as expected if Docker Machine has access to the DHCP network of RackHD.

Create a Docker host using a specific Node ID. In this case, the node already has an OS (CentOS) and we are using the default username and password to SSH into the node. An SSH key will be automatically generated and used for future connections.

```
$ docker-machine create -d rackhd --rackhd-node-id 56c61189f21f01b608b3e594 rackhdtest
Running pre-create checks...
(rackhdtest) Testing accessibility of endpoint: localhost:8080
(rackhdtest) Test Passed. localhost:8080 Monorail and Redfish API's are accessible and installation will begin
Creating machine...
(rackhdtest) Connection succeeded on: 172.31.128.16:22
(rackhdtest) Creating SSH key...
(rackhdtest) Copy public SSH key to rackhdtest [172.31.128.16]
Waiting for machine to be running, this may take a few minutes...
Detecting operating system of created instance...
Waiting for SSH to be available...
Detecting the provisioner...
Provisioning with centos...
Copying certs to the local machine directory...
Copying certs to the remote machine...
Setting Docker configuration on the remote daemon...
Checking connection to Docker...
Docker is up and running!
To see how to connect your Docker Client to the Docker Engine running on this virtual machine, run: docker-machine env rackhdtest
```

Here is an example using the name of a SKU, allowing the driver to choose a node automatically. We also have the driver install CoreOS on the node, and pass it the path to an existing SSH private key that will match the public key that gets installed on the node by RackHD.

```
$ docker-machine create --d rackhd --rackhd-sku-name SmallNode --rackhd-ssh-key ~/git/RackHD/on-http/data/rackhd_rsa --rackhd-workflow-name Graph.InstallCoreOS --rackhd-ssh-user core rackhdtest
Running pre-create checks...
(rackhdtest) Testing accessibility of endpoint: localhost:9090
(rackhdtest) Test Passed. localhost:9090 Monorail and Redfish API's are accessible and installation will begin
(rackhdtest) Looking for available node within SKU
(rackhdtest) Found a free node within SKU, Node ID: 57bdf3197ca543010074684b
Creating machine...
(rackhdtest) Connection succeeded on: 172.31.128.5:22
Waiting for machine to be running, this may take a few minutes...
Detecting operating system of created instance...
Waiting for SSH to be available...
Detecting the provisioner...
Provisioning with coreOS...
Copying certs to the local machine directory...
Copying certs to the remote machine...
Setting Docker configuration on the remote daemon...
Checking connection to Docker...
Docker is up and running!
To see how to connect your Docker Client to the Docker Engine running on this virtual machine, run: docker-machine env rackhdtest
```

Note that when using a workflow to install an OS, it takes many minutes to do the install. It can be useful to use the `--debug` flag to track progress.

---

Check out the [RackHD Vagrant + Docker Machine Example](https://github.com/emccode/machine/tree/master/rackhd) to view a complete in-depth configuration and walk-through.

[![Docker Machine Driver for RackHD Vagrant Setup and Testing](http://i.imgur.com/346xWSZ.png)](https://github.com/emccode/machine/tree/master/rackhd "RackHD Machine Driver Vagrant Setup and Testing")

## Docker Machine Functions

The functions for life cycle of machine management such as **Start**, **Stop**, **Restart**, **Kill**, and **Remove** requires the use of IPMI or other OBM solution. The driver does not need to know these credentials, rather they are configured within RackHD. Be sure these credentials are a part of the RackHD provisioning workflow when a node is being discovered.

# Licensing
Licensed under the Apache License, Version 2.0 (the “License”); you may not use this file except in compliance with the License. You may obtain a copy of the License at <http://www.apache.org/licenses/LICENSE-2.0>

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an “AS IS” BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

# Support
If you have questions relating to the project, please either post [Github Issues](https://github.com/emccode/docker-machine-driver-rackhd/issues), join our Slack channel available by signup through [community.emc.com](https://community.emccode.com) and post questions into `#support`, or reach out to the maintainers directly.  The code and documentation are released with no warranties or SLAs and are intended to be supported through a community driven process.
