# RackHD Driver for Docker Machine

Create Docker hosts with [RackHD](https://github.com/RackHD/RackHD).

## Installation

Not ready for use. Stay Tuned!

## Using the driver

To use the driver first make sure you are running at least [version
0.5.1 of `docker-machine`](https://github.com/docker/machine/releases).

```
$ docker-machine -v
docker-machine version 0.5.1 (7e8e38e)
```

Check that `docker-machine` can see the RackHD driver by asking for the driver help.

```
$ docker-machine create -d rackhd | more
Usage: docker-machine create [OPTIONS] [arg...]
```

## Create a machine.

Specify a driver with `--driver` or `-d` to include the create flags for that driver in the help text.

Options:

| Option                  |  Environment Variable | Default | Description                                     | Required? |
|-------------------------|:---------------------:|---------|-------------------------------------------------|:---------:|
| --rackhd-endpoint    |   RACKHD_ENDPOINT  |     localhost:9090    | RackHD Endpoint for API traffic           |     N     |
| --rackhd-node-id | RACKHD_NODE_ID |         | Specify Node ID, MAC Address or IP Address           |     Y     |
| --rackhd-transport    |   RACKHD_TRANSPORT  |    http     | RackHD Endpoint Transport. Specify http or https |     N     |
| --rackhd-ssh-user    |   RACKHD_SSH_USER  |    root    | SSH User Name for the node        |     N      |
| --rackhd-ssh-password     |   RACKHD_SSH_PASSWORD   |    root   | SSH Password for the node               |     N      |
| --rackhd-ssh-port      |    RACKHD_SSH_PORT   |    22    | SSH Port for the node          |      N     |

This initial version of the driver uses explicit instructions. The user must specify the Node ID from RackHD. The NodeID is characterized as a `compute` instance. Do not use `enclosure`.

Create a Docker host using the following example. The client assumes you are using a Vagant instance. Note at this time, it will fail because the nodes being PXE booted via Vagrant do not have SSH access. SSH access is possible by creating an additional NIC, but RackHD doesn't have the capability to get the IP Address and add it to its own database if it didn't give the IP. This will work best using physical hardware and a network where Docker Machine has  access to the DHCP network of RackHD

```
$ docker-machine create -d rackhd --rackhd-node-id 56c61189f21f01b608b3e594 rackhdtest
Running pre-create checks...
Creating machine...
Connection succeeded on: 172.31.128.2:22
Waiting for machine to be running, this may take a few minutes...
Machine is running, waiting for SSH to be available...
Detecting operating system of created instance...
Provisioning created instance...
Copying certs to the local machine directory...
Copying certs to the remote machine...
Setting Docker configuration on the remote daemon...
To see how to connect Docker to this machine, run: docker-machine env rackhdtest
```


# Licensing
Licensed under the Apache License, Version 2.0 (the “License”); you may not use this file except in compliance with the License. You may obtain a copy of the License at <http://www.apache.org/licenses/LICENSE-2.0>

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an “AS IS” BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

# Support
If you have questions relating to the project, please either post [Github Issues](https://github.com/emccode/docker-machine-driver-rackhd/issues), join our Slack channel available by signup through [community.emc.com](https://community.emccode.com) and post questions into `#support`, or reach out to the maintainers directly.  The code and documentation are released with no warranties or SLAs and are intended to be supported through a community driven process.