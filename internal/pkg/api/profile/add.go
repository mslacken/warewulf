package apiprofile

import (
	"fmt"

	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/util"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

/*
Adds a new profile with the given name
*/
func AddProfile(nsp *wwapiv1.ConfSetParameter) error {
	if nsp == nil {
		return fmt.Errorf("NodeSetParameter is nill")
	}
	nodeDB, err := node.New()
	if err != nil {
		return errors.Wrap(err, "Could not open database")
	}

	if util.InSlice(nodeDB.ListAllProfiles(), nsp.ConfList[0]) {
		return errors.New(fmt.Sprintf("profile with name %s already exists", nsp.ConfList[0]))
	}

	p, err := nodeDB.AddProfile(nsp.ConfList[0])
	if err != nil {
		return err
	}
	err = yaml.Unmarshal([]byte(nsp.NodeConfYaml), &p)
	if err != nil {
		return errors.Wrap(err, "failed to add profile")
	}
	err = nodeDB.Persist()
	if err != nil {
		return errors.Wrap(err, "failed to persist new profile")
	}
	return nil
}
