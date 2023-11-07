package overlay

import (
	"net"
	"os"
	"strconv"
	"time"

	warewulfconf "github.com/hpcng/warewulf/internal/pkg/config"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
)

/*
struct which contains the variables to which are available in
the templates.
*/
type TemplateStruct struct {
	Id            string
	Hostname      string
	BuildHost     string
	BuildTime     string
	BuildTimeUnix string
	BuildSource   string
	Ipaddr        string
	Ipaddr6       string
	Netmask       string
	Network       string
	NetworkCIDR   string
	Ipv6          bool
	Dhcp          warewulfconf.DHCPConf
	Nfs           warewulfconf.NFSConf
	Warewulf      warewulfconf.WarewulfConf
	Tftp          warewulfconf.TFTPConf
	Paths         warewulfconf.BuildConfig
	AllNodes      []node.NodeInfo
	ProfileMap    map[string]*node.NodeInfo
	node.NodeConf
	// backward compatiblity
	Container string
	ThisNode  *node.NodeInfo
}

/*
Initialize an TemplateStruct with the given node.NodeInfo
*/
func InitStruct(nodeInfo *node.NodeInfo) TemplateStruct {
	var tstruct TemplateStruct
	tstruct.ThisNode = nodeInfo
	controller := warewulfconf.Get()
	nodeDB, err := node.New()
	if err != nil {
		wwlog.Warn("Problems opening nodes.conf: %s", err)
	}
	tstruct.AllNodes, err = nodeDB.FindAllNodes()
	if err != nil {
		wwlog.Warn("couldn't get all nodes: %s", err)
	}
	tstruct.ProfileMap, err = nodeDB.MapAllProfiles()
	if err != nil {
		wwlog.Warn("couldn't get all profiles: %s", err)
	}
	// init some convenience vars
	tstruct.Id = nodeInfo.Id.Get()
	tstruct.Hostname = nodeInfo.Id.Get()
	tstruct.Nfs = *controller.NFS
	tstruct.Dhcp = *controller.DHCP
	tstruct.Tftp = *controller.TFTP
	tstruct.Warewulf = *controller.Warewulf
	tstruct.Ipaddr = controller.Ipaddr
	tstruct.Ipaddr6 = controller.Ipaddr6
	tstruct.Netmask = controller.Netmask
	tstruct.Network = controller.Network
	netaddrStruct := net.IPNet{IP: net.ParseIP(controller.Network), Mask: net.IPMask(net.ParseIP(controller.Netmask))}
	tstruct.NetworkCIDR = netaddrStruct.String()
	if controller.Ipaddr6 != "" {
		tstruct.Ipv6 = true
	} else {
		tstruct.Ipv6 = false
	}
	hostname, _ := os.Hostname()
	tstruct.BuildHost = hostname
	dt := time.Now()
	tstruct.BuildTime = dt.Format("01-02-2006 15:04:05 MST")
	tstruct.BuildTimeUnix = strconv.FormatInt(dt.Unix(), 10)
	tstruct.NodeConf.Tags = map[string]string{}
	tstruct.NodeConf.GetFrom(*nodeInfo)
	// FIXME: Set ipCIDR address at this point, will fail with
	// invalid ipv4 addr
	for _, network := range tstruct.NodeConf.NetDevs {
		ipCIDR := net.IPNet{
			IP:   net.ParseIP(network.Ipaddr),
			Mask: net.IPMask(net.ParseIP(network.Netmask))}
		network.IpCIDR = ipCIDR.String()
	}
	// backward compatibilty
	tstruct.Container = tstruct.NodeConf.ContainerName

	return tstruct

}
