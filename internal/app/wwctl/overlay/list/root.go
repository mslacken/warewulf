package list

import (
	"github.com/spf13/cobra"
)

var (
	baseCmd = &cobra.Command{
		DisableFlagsInUseLine: true,
		Use:   "list [OPTIONS] {system|runtime} [OVERLAY_NAME]",
		Short: "List Warewulf Overlays and files",
		Long: "This command displays information about all Warewulf overlays or the specified\n" +
			"OVERLAY_NAME. It also supports listing overlay content information.",
		RunE:      CobraRunE,
		Args:      cobra.MinimumNArgs(1),
		Aliases:   []string{"ls"},
		ValidArgs: []string{"system", "runtime"},
	}
	ListContents bool
	ListLong     bool
)

func init() {
	baseCmd.PersistentFlags().BoolVarP(&ListContents, "all", "a", false, "List the contents of overlays")
	baseCmd.PersistentFlags().BoolVarP(&ListLong, "long", "l", false, "List 'long' of all overlay contents")

}

// GetRootCommand returns the root cobra.Command for the application.
func GetCommand() *cobra.Command {
	return baseCmd
}
