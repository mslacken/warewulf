package set

import (
	"log"

	"github.com/hpcng/warewulf/internal/pkg/container"
	"github.com/hpcng/warewulf/internal/pkg/kernel"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/overlay"
	"github.com/spf13/cobra"
)

type variables struct {
	setNodeAll bool
	setYes     bool
	setForce   bool
	nodeConf   node.NodeConf
	nodeDel    node.NodeConfDel
	nodeAdd    node.NodeConfAdd
	converters []func() error
}

func GetCommand() *cobra.Command {
	vars := variables{}
	vars.nodeConf = node.NewConf("")
	baseCmd := &cobra.Command{
		DisableFlagsInUseLine: true,
		Use:                   "set [OPTIONS] PATTERN [PATTERN ...]",
		Short:                 "Configure node properties",
		Long:                  "This command sets configuration properties for nodes matching PATTERN.\n\nNote: use the string 'UNSET' to remove a configuration",
		Args:                  cobra.MinimumNArgs(0),
		RunE:                  CobraRunE(&vars),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			nodeDB, _ := node.New()
			nodes, _ := nodeDB.FindAllNodes()
			var node_names []string
			for _, node := range nodes {
				node_names = append(node_names, node.Id())
			}
			return node_names, cobra.ShellCompDirectiveNoFileComp
		},
	}

	vars.converters = vars.nodeConf.CreateFlags(baseCmd, []string{})
	vars.converters = vars.nodeAdd.CreateAddFlags(baseCmd)
	vars.converters = vars.nodeDel.CreateDelFlags(baseCmd)
	baseCmd.PersistentFlags().BoolVarP(&vars.setNodeAll, "all", "a", false, "Set all nodes")
	baseCmd.PersistentFlags().BoolVarP(&vars.setYes, "yes", "y", false, "Set 'yes' to all questions asked")
	baseCmd.PersistentFlags().BoolVarP(&vars.setForce, "force", "f", false, "Force configuration (even on error)")
	// register the command line completions
	if err := baseCmd.RegisterFlagCompletionFunc("container", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		list, _ := container.ListSources()
		return list, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		log.Println(err)
	}
	if err := baseCmd.RegisterFlagCompletionFunc("kerneloverride", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		list, _ := kernel.ListKernels()
		return list, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		log.Println(err)
	}
	if err := baseCmd.RegisterFlagCompletionFunc("runtime", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		list, _ := overlay.FindOverlays()
		return list, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		log.Println(err)
	}
	if err := baseCmd.RegisterFlagCompletionFunc("wwinit", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		list, _ := overlay.FindOverlays()
		return list, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		log.Println(err)
	}
	if err := baseCmd.RegisterFlagCompletionFunc("profile", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var list []string
		nodeDB, _ := node.New()
		profiles, _ := nodeDB.FindAllProfiles()
		for _, profile := range profiles {
			list = append(list, profile.Id())
		}
		return list, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		log.Println(err)
	}

	return baseCmd
}
