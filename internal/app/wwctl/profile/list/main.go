package list

import (
	"fmt"
	"os"
	"strings"

	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/spf13/cobra"
)

func CobraRunE(cmd *cobra.Command, args []string) error {

	nodeDB, err := node.New()
	if err != nil {
		wwlog.Printf(wwlog.ERROR, "Could not open node configuration: %s\n", err)
		os.Exit(1)
	}

	profiles, err := nodeDB.FindAllProfiles()
	if err != nil {
		wwlog.Printf(wwlog.ERROR, "Could not find all nodes: %s\n", err)
		os.Exit(1)
	}

	if ShowAll {
		for _, profile := range node.FilterByName(profiles, args) {
			fmt.Printf("################################################################################\n")
			fmt.Printf("%-20s %-18s %s\n", "PROFILE NAME", "FIELD", "VALUE")
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "Id", profile.Id.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "Comment", profile.Comment.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "Cluster", profile.ClusterName.Print())

			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "Container", profile.ContainerName.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "Kernel", profile.KernelVersion.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "KernelArgs", profile.KernelArgs.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "Init", profile.Init.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "Root", profile.Root.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "DeviceConf", profile.DeviceConf.Print())

			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "RuntimeOverlay", profile.RuntimeOverlay.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "SystemOverlay", profile.SystemOverlay.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "Ipxe", profile.Ipxe.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "IpmiIpaddr", profile.IpmiIpaddr.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "IpmiNetmask", profile.IpmiNetmask.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "IpmiPort", profile.IpmiPort.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "IpmiGateway", profile.IpmiGateway.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "IpmiUserName", profile.IpmiUserName.Print())
			fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "IpmiInterface", profile.IpmiInterface.Print())

			for name, netdev := range profile.NetDevs {
				fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), name+":IPADDR", netdev.Ipaddr.Print())
				fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), name+":NETMASK", netdev.Netmask.Print())
				fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), name+":GATEWAY", netdev.Gateway.Print())
				fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), name+":HWADDR", netdev.Hwaddr.Print())
				fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), name+":TYPE", netdev.Hwaddr.Print())
				fmt.Printf("%-20s %-18s %t\n", profile.Id.Get(), name+":DEFAULT", netdev.Default.PrintB())
			}

			for keyname, key := range profile.Keys {
				fmt.Printf("%-20s %-18s %s\n", profile.Id.Get(), "Keys."+keyname, key.Print())
			}
		}
	} else {
		fmt.Printf("%-20s %s\n", "PROFILE NAME", "COMMENT/DESCRIPTION")
		fmt.Println(strings.Repeat("=", 80))

		for _, profile := range node.FilterByName(profiles, args) {
			fmt.Printf("%-20s %s\n", profile.Id.Print(), profile.Comment.Print())
		}
	}

	return nil
}
