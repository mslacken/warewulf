package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func newConstructorPrimaryNetworkTest() NodeYaml {
	var data = `
nodeprofiles:
  default:
    comment: This profile is automatically included for each node
nodes:
  test_node1:
    network devices:
      net0:
        device: eth0
  test_node2:
    primary network: net1
    network devices:
      net0:
        device: eth0
      net1:
        device: eth1
  test_node3:
    network devices:
      net0:
        device: eth0
      net1:
        device: eth1
  test_node4:
    primary network: net3
    network devices:
      net0:
        device: eth0
      net1:
        device: eth1
  `
	var ret NodeYaml
	_ = yaml.Unmarshal([]byte(data), &ret)
	return ret
}

func Test_Primary_Network(t *testing.T) {
	c := newConstructorPrimaryNetworkTest()
	test_node1, err := c.GetNode("test_node1")
	assert.NoError(t, err)
	test_node2, err := c.GetNode("test_node2")
	assert.NoError(t, err)
	test_node3, err := c.GetNode("test_node3")
	assert.NoError(t, err)
	test_node4, err := c.GetNode("test_node4")
	assert.NoError(t, err)
	t.Run("Primary network with one network, nothing set", func(t *testing.T) {
		if test_node1.PrimaryNetDev != "net0" {
			t.Errorf("primary network isn't net0 but: %s", test_node1.PrimaryNetDev)
		}
		if !test_node1.NetDevs["net0"].primary {
			t.Errorf("primary flag isn't set for net0")
		}
	})
	t.Run("Primary network with two networks, primary is net1", func(t *testing.T) {
		if test_node2.PrimaryNetDev != "net1" {
			t.Errorf("primary network isn't net1 but: %s", test_node2.PrimaryNetDev)
		}
		if test_node2.NetDevs["net0"].primary {
			t.Errorf("primary flag is set for net0")
		}
		if !test_node2.NetDevs["net1"].primary {
			t.Errorf("primary flag isn't set for net1")
		}
	})
	t.Run("Primary network with two networks, primary isn't set", func(t *testing.T) {
		if test_node3.PrimaryNetDev != "net0" && test_node3.PrimaryNetDev != "net1" {
			t.Errorf("network wasn't sanitzied")
		}
		if test_node3.NetDevs["net0"].primary == test_node3.NetDevs["net1"].primary {
			t.Errorf("primary flag isn't set at all")
		}
	})
	// debateable what result we await here, on refactoring primary network w
	// will be one of the valid networks
	t.Run("Primary network with two networks, primary available", func(t *testing.T) {
		if test_node4.PrimaryNetDev == "net3" {
			t.Errorf("primary network isn net3, although node hasn't this network")
		}
		if test_node4.NetDevs["net0"].primary == test_node4.NetDevs["net1"].primary {
			t.Errorf("node primary flag isn't set")
		}
	})
}

var findDiscoverableNodeTests = []struct {
	description          string
	discoverable_nodes   []string
	discovered_node      string
	discovered_interface string
	succeed              bool
}{
	{"no discoverable nodes", []string{}, "", "", false},
	{"all nodes discoverable", []string{"test_node1", "test_node2", "test_node3", "test_node4"}, "test_node1", "net0", true},
	{"discover primary", []string{"test_node2"}, "test_node2", "net1", true},
	{"discovery without primary", []string{"test_node3"}, "test_node3", "net0", true},
}

func Test_FindDiscoverableNode(t *testing.T) {
	for _, tt := range findDiscoverableNodeTests {
		t.Run(tt.description, func(t *testing.T) {
			config := newConstructorPrimaryNetworkTest()
			for _, node := range tt.discoverable_nodes {
				config.Nodes[node].Discoverable = true
			}
			discovered_node, discovered_interface, err := config.FindDiscoverableNode()
			if !tt.succeed {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.discovered_node, discovered_node)
				assert.Equal(t, tt.discovered_interface, discovered_interface)
			}
		})
	}
}
