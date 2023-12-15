package node

import (
	"errors"
	"net"
	"strings"
)

/*
Gets a node by its hardware(mac) address
*/
func (config *NodeYaml) FindByHwaddr(hwa string) (NodeConf, error) {
	if _, err := net.ParseMAC(hwa); err != nil {
		return NodeConf{}, errors.New("invalid hardware address: " + hwa)
	}
	nodeList, _ := config.FindAllNodes()
	for _, node := range nodeList {
		for _, dev := range node.NetDevs {
			if strings.EqualFold(dev.Hwaddr, hwa) {
				return node, nil
			}
		}
	}

	return NodeConf{}, ErrNotFound
}

/*
Find a node by its ip address
*/
func (config *NodeYaml) FindByIpaddr(ipaddr string) (NodeConf, error) {
	addr := net.ParseIP(ipaddr)
	if addr == nil {
		return NodeConf{}, errors.New("invalid IP:" + ipaddr)
	}
	nodeList, err := config.FindAllNodes()
	if err != nil {
		return NodeConf{}, err
	}
	for _, node := range nodeList {
		for _, dev := range node.NetDevs {
			devaddr := net.ParseIP(dev.Ipaddr)
			if devaddr.Equal(addr) {
				return node, nil
			}
		}
	}

	return NodeConf{}, ErrNotFound
}

/*
// Return just the node list as string slice
func (config *NodeYaml) NodeList() []string {
	ret := make([]string, len(config.Nodes))
	for key := range config.Nodes {
		ret = append(ret, key)
	}
	return ret
}
*/
