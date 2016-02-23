package main

import (
	"github.com/docker/machine/libmachine/drivers/plugin"
	"github.com/emccode/docker-machine-driver-rackhd"
)

func main() {
	plugin.RegisterDriver(new(rackhd.Driver))
}
