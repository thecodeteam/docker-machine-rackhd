# Docker Machine Driver for RackHD

Use [Docker Machine](https://github.com/docker/machine) to create Docker hosts with [RackHD](https://github.com/RackHD/RackHD).

## Installation

**Linux & Mac OSX**:
```
curl -L https://github.com/emccode/docker-machine-rackhd/releases/download/v0.0.1/docker-machine-driver-rackhd.`uname -s`-`uname -m` >/usr/local/bin/docker-machine-driver-rackhd &&  chmod +x /usr/local/bin/docker-machine-driver-rackhd
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

| Option                  |  Environment Variable | Default | Description                                     | Required? |
|-------------------------|:---------------------:|---------|-------------------------------------------------|:---------:|
| --rackhd-endpoint    |   RACKHD_ENDPOINT  |     localhost:8080    | RackHD Endpoint for API traffic           |     N     |
| --rackhd-node-id | RACKHD_NODE_ID |         | Specify Node ID, MAC Address or IP Address           |     Y     |
| --rackhd-transport    |   RACKHD_TRANSPORT  |    http     | RackHD Endpoint Transport. Specify http or https |     N     |
| --rackhd-ssh-user    |   RACKHD_SSH_USER  |    root    | SSH User Name for the node        |     N      |
| --rackhd-ssh-password     |   RACKHD_SSH_PASSWORD   |    root   | SSH Password for the node               |     N      |
| --rackhd-ssh-port      |    RACKHD_SSH_PORT   |    22    | SSH Port for the node          |      N     |

This initial version of the driver uses explicit creation instructions. The user must specify the Node ID from RackHD. The NodeID is characterized as a `compute` instance. Do not use `enclosure`.

Create a Docker host using the following example. This will function as expected if Docker Machine has access to the DHCP network of RackHD.

```
$ docker-machine create -d rackhd --rackhd-node-id 56c61189f21f01b608b3e594 rackhdtest
Running pre-create checks...
(rackhdtest) Testing accessibility of endpoint: localhost:8080
(rackhdtest) Test Passed. localhost:8080 is accessbile and installation will begin
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

Check out the [RackHD Vagrant + Docker Machine Example](https://github.com/emccode/machine/tree/master/rackhd) to view a complete in-depth configuration and walk-through.

# Licensing
Licensed under the Apache License, Version 2.0 (the “License”); you may not use this file except in compliance with the License. You may obtain a copy of the License at <http://www.apache.org/licenses/LICENSE-2.0>

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an “AS IS” BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

# Support
If you have questions relating to the project, please either post [Github Issues](https://github.com/emccode/docker-machine-driver-rackhd/issues), join our Slack channel available by signup through [community.emc.com](https://community.emccode.com) and post questions into `#support`, or reach out to the maintainers directly.  The code and documentation are released with no warranties or SLAs and are intended to be supported through a community driven process.