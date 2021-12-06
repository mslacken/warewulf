package overlay

import (
	"github.com/hpcng/warewulf/internal/app/wwctl/overlay/build"
	"github.com/hpcng/warewulf/internal/app/wwctl/overlay/chmod"
	"github.com/hpcng/warewulf/internal/app/wwctl/overlay/chown"
	"github.com/hpcng/warewulf/internal/app/wwctl/overlay/create"
	"github.com/hpcng/warewulf/internal/app/wwctl/overlay/delete"
	"github.com/hpcng/warewulf/internal/app/wwctl/overlay/edit"
	"github.com/hpcng/warewulf/internal/app/wwctl/overlay/imprt"
	"github.com/hpcng/warewulf/internal/app/wwctl/overlay/list"
	"github.com/hpcng/warewulf/internal/app/wwctl/overlay/mkdir"
	"github.com/hpcng/warewulf/internal/app/wwctl/overlay/show"
	"github.com/spf13/cobra"
)

var (
	baseCmd = &cobra.Command{
		DisableFlagsInUseLine: true,
		Use:   "overlay COMMAND [OPTIONS]",
		Short: "Warewulf Overlay Management",
		Long:  "Management interface for Warewulf overlays",
	}
)

func init() {
	baseCmd.AddCommand(list.GetCommand())
	baseCmd.AddCommand(show.GetCommand())
	baseCmd.AddCommand(create.GetCommand())
	baseCmd.AddCommand(edit.GetCommand())
	baseCmd.AddCommand(delete.GetCommand())
	baseCmd.AddCommand(mkdir.GetCommand())
	baseCmd.AddCommand(build.GetCommand())
	baseCmd.AddCommand(imprt.GetCommand())
	baseCmd.AddCommand(chmod.GetCommand())
	baseCmd.AddCommand(chown.GetCommand())
}

// GetRootCommand returns the root cobra.Command for the application.
func GetCommand() *cobra.Command {
	return baseCmd
}
