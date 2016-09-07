package main

import (
	"github.com/codedellemc/docker-machine-rackhd"
	"github.com/docker/machine/libmachine/drivers/plugin"
)

func main() {
	plugin.RegisterDriver(new(rackhd.Driver))
}
