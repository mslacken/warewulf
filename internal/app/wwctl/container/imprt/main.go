package imprt

import (
	"github.com/hpcng/warewulf/internal/pkg/container"

	apicontainer "github.com/hpcng/warewulf/internal/pkg/api/container"
	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/spf13/cobra"
)

func CobraRunE(cmd *cobra.Command, args []string) (err error) {

	// Shim in a name if none given.
	name := ""
	if len(args) == 2 {
		name = args[1]
	}
	if list, _ := container.ListSources(); len(list) == 0 {
		SetDefault = true
	}

	cip := &wwapiv1.ContainerImportParameter{
		Source:   args[0],
		Name:     name,
		Force:    SetForce,
		Update:   SetUpdate,
		Build:    SetBuild,
		Default:  SetDefault,
		SyncUser: SyncUser,
	}

	_, err = apicontainer.ContainerImport(cip)
	return
}
