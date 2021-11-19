package buildhybrid

import (
	"github.com/hpcng/warewulf/internal/pkg/container"
	"github.com/spf13/cobra"
)

var (
	baseCmd = &cobra.Command{
		Use:   "hybridbuild [flags] [container name]...",
		Short: "Build a bootable hybrid VNFS image",
		Long:  "This command will build a bootable hybrid VNFS image from an existing container image. A hybrid container with the '-hybrid' suffix will be created.",
		RunE:  CobraRunE,
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			list, _ := container.ListSources()
			return list, cobra.ShellCompDirectiveNoFileComp
		},
	}
	SetDefault     bool
	AdditionalExec []string
)

func init() {
	baseCmd.PersistentFlags().BoolVar(&SetDefault, "setdefault", false, "Set the resulting hybrid container for the default profile")
	baseCmd.PersistentFlags().StringArrayVar(&AdditionalExec, "execs", nil, "Add additional exectuteables which must be present in container, like mkfs.btrfs")
}

// GetRootCommand returns the root cobra.Command for the application.
func GetCommand() *cobra.Command {
	return baseCmd
}
