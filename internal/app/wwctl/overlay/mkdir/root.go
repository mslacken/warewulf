package mkdir

import (
	"github.com/spf13/cobra"
)

var (
	baseCmd = &cobra.Command{
		Use:   "mkdir [flags] (overlay kind) (overlay name) (directory path)",
		Short: "Create a new directory within an Overlay",
		Long:  "This command will allow you to create a new file within a given Warewulf overlay.",
		RunE:  CobraRunE,
		Args:  cobra.MinimumNArgs(3),
	}
	PermMode int32
)

func init() {
	baseCmd.PersistentFlags().Int32VarP(&PermMode, "mode", "m", 0755, "Permission mode for directory")
}

// GetRootCommand returns the root cobra.Command for the application.
func GetCommand() *cobra.Command {
	return baseCmd
}
