package poweron

import (
	"fmt"
	"os"

	"github.com/hpcng/warewulf/internal/pkg/batch"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/power"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/hpcng/warewulf/pkg/hostlist"
	"github.com/spf13/cobra"
)

func CobraRunE(cmd *cobra.Command, args []string) error {
	var returnErr error = nil

	nodeDB, err := node.New()
	if err != nil {
		wwlog.Error("Could not open node configuration: %s", err)
		os.Exit(1)
	}

	nodes, err := nodeDB.FindAllNodes()
	if err != nil {
		wwlog.Error("Could not get node list: %s", err)
		os.Exit(1)
	}

	if len(args) > 0 {
		nodes = node.FilterByName(nodes, hostlist.Expand(args))
	} else {
		//nolint:errcheck
		cmd.Usage()
		os.Exit(1)
	}

	if len(nodes) == 0 {
		fmt.Printf("No nodes found\n")
		os.Exit(1)
	}

	batchpool := batch.New(50)
	jobcount := len(nodes)
	results := make(chan power.IPMI, jobcount)

	for _, node := range nodes {

		if node.Ipmi.Ipaddr == "" {
			wwlog.Error("%s: No IPMI IP address", node.Id())
			continue
		}
		var ipmiInterface = "lan"
		if node.Ipmi.Interface != "" {
			ipmiInterface = node.Ipmi.Interface
		}
		var ipmiPort = "623"
		if node.Ipmi.Port != "" {
			ipmiPort = node.Ipmi.Port
		}
		ipmiCmd := power.IPMI{
			NodeName:  node.Id(),
			HostName:  node.Ipmi.Ipaddr,
			Port:      ipmiPort,
			User:      node.Ipmi.UserName,
			Password:  node.Ipmi.Password,
			Interface: ipmiInterface,
			AuthType:  "MD5",
		}

		batchpool.Submit(func() {
			//nolint:errcheck
			ipmiCmd.PowerOn()
			results <- ipmiCmd
		})

	}

	batchpool.Run()

	close(results)

	for result := range results {

		out, err := result.Result()

		if err != nil {
			wwlog.Error("%s: %s", result.NodeName, out)
			returnErr = err
			continue
		}

		fmt.Printf("%s: %s\n", result.NodeName, out)

	}

	return returnErr
}
