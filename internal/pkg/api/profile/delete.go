package apiprofile

import (
	"fmt"

	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/warewulfd"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/hpcng/warewulf/pkg/hostlist"
	"github.com/pkg/errors"
)

// ProfileDelete adds profile deletion for management by Warewulf.
func ProfileDelete(ndp *wwapiv1.NodeDeleteParameter) (err error) {
	nodeDB, profileList, err := ProfileDeleteParameterCheck(ndp, false)
	if err != nil {
		return
	}
	if nodeDB.StringHash() != ndp.Hash && !ndp.Force {
		return fmt.Errorf("got wrong hash, not modifying profile database")
	}
	for _, p := range profileList {
		delete(nodeDB.NodeProfiles, p)
		wwlog.Verbose("Deleting profile: %s", p)
	}

	err = nodeDB.Persist()
	if err != nil {
		return errors.Wrap(err, "failed to persist nodedb")
	}
	err = warewulfd.DaemonReload()
	if err != nil {
		return errors.Wrap(err, "failed to reload warewulf daemon")
	}
	return
}

// ProfileDeleteParameterCheck does error checking on ProfileDeleteParameter.
// Output to the console if console is true.
// Returns the profiles to delete.
func ProfileDeleteParameterCheck(ndp *wwapiv1.NodeDeleteParameter, console bool) (nodeDB node.NodeYaml, profileList []string, err error) {

	if ndp == nil {
		err = fmt.Errorf("profileDeleteParameter is nil")
		return
	}

	nodeDB, err = node.New()
	if err != nil {
		wwlog.Error("failed to open node database: %s\n", err)
		return
	}
	profileList = nodeDB.ListAllProfiles()
	profileArgs := hostlist.Expand(ndp.NodeNames)
	for _, r := range profileArgs {
		match := false
		for _, p := range profileList {
			if p == r {
				profileList = append(profileList, p)
				match = true
			}
		}

		if !match {
			wwlog.Error("no match for profile: %s", r)
		}
	}

	if len(profileList) == 0 {
		wwlog.Warn("no profiles found\n")
	}
	return
}
