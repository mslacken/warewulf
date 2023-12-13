package apinode

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/pkg/hostlist"
)

/*
NodeList lists all to none of the nodes managed by Warewulf. Returns
a formated string slice, with each line as separate string
*/
func NodeList(nodeGet *wwapiv1.GetNodeList) (nodeList wwapiv1.NodeList, err error) {
	// nil is okay for nodeNames
	nodeDB, err := node.New()
	if err != nil {
		return
	}
	nodes, err := nodeDB.FindAllNodes()
	if err != nil {
		return
	}
	nodeGet.Nodes = hostlist.Expand(nodeGet.Nodes)
	sort.Strings(nodeGet.Nodes)
	if nodeGet.Type == wwapiv1.GetNodeList_Simple {
		nodeList.Output = append(nodeList.Output,
			fmt.Sprintf("%s:=:%s:=:%s", "NODE NAME", "PROFILES", "NETWORK"))
		for _, n := range node.FilterByName(nodes, nodeGet.Nodes) {
			var netNames []string
			for k := range n.NetDevs {
				netNames = append(netNames, k)
			}
			sort.Strings(netNames)
			nodeList.Output = append(nodeList.Output,
				fmt.Sprintf("%s:=:%s:=:%s", n.Id, n.Profiles, strings.Join(netNames, ", ")))
		}
	} else if nodeGet.Type == wwapiv1.GetNodeList_Network {
		nodeList.Output = append(nodeList.Output,
			fmt.Sprintf("%s:=:%s:=:%s:=:%s:=:%s:=:%s", "NODE NAME", "NAME", "HWADDR", "IPADDR", "GATEWAY", "DEVICE"))
		for _, n := range node.FilterByName(nodes, nodeGet.Nodes) {
			if len(n.NetDevs) > 0 {
				for name := range n.NetDevs {
					nodeList.Output = append(nodeList.Output,
						fmt.Sprintf("%s:=:%s:=:%s:=:%s:=:%s:=:%s", n.Id(), name,
							n.NetDevs[name].Hwaddr,
							n.NetDevs[name].Ipaddr,
							n.NetDevs[name].Gateway,
							n.NetDevs[name].Device))
				}
			} else {
				fmt.Printf("%s:=:%s:=:%s:=:%s:=:%s:=:%s", n.Id, "--", "--", "--", "--", "--")
			}
		}
	} else if nodeGet.Type == wwapiv1.GetNodeList_Ipmi {
		nodeList.Output = append(nodeList.Output,
			fmt.Sprintf("%s:=:%s:=:%s:=:%s:=:%s", "NODE NAME", "IPMI IPADDR", "IPMI PORT", "IPMI USERNAME", "IPMI INTERFACE"))
		for _, n := range node.FilterByName(nodes, nodeGet.Nodes) {
			nodeList.Output = append(nodeList.Output,
				fmt.Sprintf("%s:=:%s:=:%s:=:%s:=:%s:=:%s", n.Id,
					n.Ipmi.Ipaddr,
					n.Ipmi.Port,
					n.Ipmi.UserName,
					n.Ipmi.Interface,
					n.Ipmi.EscapeChar))
		}
	} else if nodeGet.Type == wwapiv1.GetNodeList_Long {
		nodeList.Output = append(nodeList.Output,
			fmt.Sprintf("%s:=:%s:=:%s:=:%s", "NODE NAME", "KERNEL OVERRIDE", "CONTAINER", "OVERLAYS (S/R)"))
		for _, n := range node.FilterByName(nodes, nodeGet.Nodes) {
			nodeList.Output = append(nodeList.Output,
				fmt.Sprintf("%s:=:%s:=:%s:=:%s", n.Id,
					n.Kernel.Override,
					n.ContainerName,
					strings.Join(n.SystemOverlay, ",")+"/"+strings.Join(n.RuntimeOverlay, ",")))
		}
	} else if nodeGet.Type == wwapiv1.GetNodeList_All || nodeGet.Type == wwapiv1.GetNodeList_FullAll {
		for _, n := range node.FilterByName(nodes, nodeGet.Nodes) {
			nodeList.Output = append(nodeList.Output,
				fmt.Sprintf("%s:=:%s:=:%s:=:%s", "NODE", "FIELD", "PROFILE", "VALUE"))
			fields := nodeDB.GetFields(n, nodeGet.Type == wwapiv1.GetNodeList_All)
			for _, f := range fields {
				nodeList.Output = append(nodeList.Output,
					fmt.Sprintf("%s:=:%s:=:%s:=:%s", n.Id, f.Field, f.Source, f.Value))
			}
		}
	}
	return
}
