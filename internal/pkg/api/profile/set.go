package apiprofile

import (
	"fmt"

	apinode "github.com/hpcng/warewulf/internal/pkg/api/node"
	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/warewulfd"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/pkg/errors"
)

// NodeSet is the wwapiv1 implmentation for updating nodeinfo fields.
func ProfileSet(set *wwapiv1.ConfSetParameter) (err error) {
	if set == nil {
		return fmt.Errorf("ProfileAddParameter is nil")
	}
	nodeDB, _, err := ProfileSetParameterCheck(set)
	if err != nil {
		return errors.Wrap(err, "profile set parameters are wrong")
	}
	if err = nodeDB.Persist(); err != nil {
		return err
	}
	if err = warewulfd.DaemonReload(); err != nil {
		return err
	}
	return
}

/*
NodeSetParameterCheck does error checking and returns a modified
NodeYml which than can be persisted
*/
func ProfileSetParameterCheck(set *wwapiv1.ConfSetParameter) (nodeDB node.NodeYaml, count uint, err error) {
	nodeDB, err = node.New()
	if err != nil {
		wwlog.Error("Could not open configuration: %s", err)
		return
	}
	profiles := nodeDB.ListAllProfiles()
	count, err = apinode.AbstractSetParameterCheck(set, nodeDB.NodeProfiles, profiles)
	return nodeDB, count, err
}
