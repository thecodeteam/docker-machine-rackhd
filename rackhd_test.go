package rackhd

import (
	"testing"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/stretchr/testify/assert"
)

func TestDriverName(t *testing.T) {
	// create the Driver
	d := NewDriver("default", "path")

	if "rackhd" != d.DriverName() {
		t.FailNow()
	}
}

func testSetDefaults(t *testing.T) {
	// create the Driver
	d := NewDriver("default", "path")

	checkFlags := &drivers.CheckDriverOptions{
		FlagsValues: map[string]interface{}{
			"rackhd-sku-id": "aabbccdd",
		},
		CreateFlags: d.GetCreateFlags(),
	}

	err := d.SetConfigFromFlags(checkFlags)

	assert.NoError(t, err)
	assert.Empty(t, checkFlags.InvalidFlags)

	assert.Equal(t, "http", d.Transport)
	assert.Equal(t, "root", d.SSHUser)
	assert.Equal(t, "root", d.SSHPassword)
	assert.Equal(t, 22, d.SSHPort)

	// Not part of the default, but check to see that it's set
	assert.Equal(t, "aabbccdd", d.SkuID)
}

func TestSetEndpoint(t *testing.T) {
	// create the Driver
	d := NewDriver("default", "path")

	checkFlags := &drivers.CheckDriverOptions{
		FlagsValues: map[string]interface{}{
			"rackhd-endpoint": "localhost:9090",
			"rackhd-node-id":  "aabbccdd",
		},
		CreateFlags: d.GetCreateFlags(),
	}

	err := d.SetConfigFromFlags(checkFlags)

	assert.NoError(t, err)
	assert.Empty(t, checkFlags.InvalidFlags)

	assert.Equal(t, "localhost:9090", d.Endpoint)
}

func TestNodeOrSkuIDRequired(t *testing.T) {
	// create the Driver
	d := NewDriver("default", "path")

	checkFlags := &drivers.CheckDriverOptions{
		FlagsValues: map[string]interface{}{
			"rackhd-endpoint": "localhost:9090",
		},
		CreateFlags: d.GetCreateFlags(),
	}

	err := d.SetConfigFromFlags(checkFlags)

	assert.Error(t, err, "Should error if no SKU or Node ID is given")
}

func TestOnlyNodeOrSkuIDAllowed(t *testing.T) {
	// create the Driver
	d := NewDriver("default", "path")

	checkFlags := &drivers.CheckDriverOptions{
		FlagsValues: map[string]interface{}{
			"rackhd-node-id": "aabbccdd",
			"rackhd-sku-id":  "eeffgghh",
		},
		CreateFlags: d.GetCreateFlags(),
	}

	err := d.SetConfigFromFlags(checkFlags)

	assert.Error(t, err, "Should error if both SKU and Node ID is given")
}

func TestOnlyNodeOrSkuNameAllowed(t *testing.T) {
	// create the Driver
	d := NewDriver("default", "path")

	checkFlags := &drivers.CheckDriverOptions{
		FlagsValues: map[string]interface{}{
			"rackhd-node-id":  "aabbccdd",
			"rackhd-sku-name": "test sku",
		},
		CreateFlags: d.GetCreateFlags(),
	}

	err := d.SetConfigFromFlags(checkFlags)

	assert.Error(t, err, "Should error if both SKU Name and Node ID are given")
}

func TestOnlySkuNameOrSkuIDAllowed(t *testing.T) {
	// create the Driver
	d := NewDriver("default", "path")

	checkFlags := &drivers.CheckDriverOptions{
		FlagsValues: map[string]interface{}{
			"rackhd-sku-id":   "aabbccdd",
			"rackhd-sku-name": "test sku",
		},
		CreateFlags: d.GetCreateFlags(),
	}

	err := d.SetConfigFromFlags(checkFlags)

	assert.Error(t, err, "Should error if both SKU Name and ID are given")
}
