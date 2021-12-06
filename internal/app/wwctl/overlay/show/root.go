package show

import (
	"github.com/spf13/cobra"
)

var (
	baseCmd = &cobra.Command{
		Use:   "show [flags] (overlay kind) (overlay name) (overlay file)",
		Short: "Show (cat) a file within a Warewulf Overlay",
		Long: "This command will output the contents of a file within a given\n" +
			"Warewulf overlay.",
		RunE:    CobraRunE,
		Aliases: []string{"cat"},
		Args:    cobra.ExactArgs(3),
	}
)

func init() {
}

// GetRootCommand returns the root cobra.Command for the application.
func GetCommand() *cobra.Command {
	return baseCmd
}
