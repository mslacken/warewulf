package imprt

import "github.com/spf13/cobra"

var (
	baseCmd = &cobra.Command{
		DisableFlagsInUseLine: true,
		Use:                   "import [OPTIONS] SOURCE [NAME]",
		Short:                 "Import a container into Warewulf",
		Long: `This command will pull and import a container into Warewulf from SOURCE,
optionally renaming it to NAME. The SOURCE must be in a supported URI format. Formats
are:
 * docker://registry.example.org/example:latest
 * docker-daemon://example:latest
 * file://path/to/archive/tar/ball
 * /path/to/archive/tar/ball
 * /path/to/chroot/
Imported containers are used to create bootable VNFS images.`,
		Example: "wwctl container import docker://ghcr.io/hpcng/warewulf-rockylinux:8 rockylinux-8",
		RunE:    CobraRunE,
		Args:    cobra.MinimumNArgs(1),
	}
	SetForce   bool
	SetUpdate  bool
	SetBuild   bool
	SetDefault bool
	SyncUser   bool
)

func init() {
	baseCmd.PersistentFlags().BoolVarP(&SetForce, "force", "f", false, "Force overwrite of an existing container")
	baseCmd.PersistentFlags().BoolVarP(&SetUpdate, "update", "u", false, "Update and overwrite an existing container")
	baseCmd.PersistentFlags().BoolVarP(&SetBuild, "build", "b", false, "Build container when after pulling")
	baseCmd.PersistentFlags().BoolVar(&SetDefault, "setdefault", false, "Set this container for the default profile")
	baseCmd.PersistentFlags().BoolVar(&SyncUser, "syncuser", false, "Synchronize UIDs/GIDs from host to container")
}

// GetRootCommand returns the root cobra.Command for the application.
func GetCommand() *cobra.Command {
	return baseCmd
}
