package delete

import (
	"fmt"

	apiNode "github.com/hpcng/warewulf/internal/pkg/api/node"
	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/hpcng/warewulf/internal/pkg/api/util"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/spf13/cobra"
)

func CobraRunE(cmd *cobra.Command, args []string) (err error) {

	ndp := wwapiv1.NodeDeleteParameter{
		Force:     SetForce,
		NodeNames: args,
	}

	if !SetYes {
		var nodeList []node.NodeConf
		// The checks run twice in the prompt case.
		// Avoiding putting in a blocking prompt in an API.
		nodeList, err = apiNode.NodeDeleteParameterCheck(&ndp, false)
		if err != nil {
			return
		}
		if len(nodeList) == 0 {
			return
		}
		yes := util.ConfirmationPrompt(fmt.Sprintf("Are you sure you want to delete %d nodes(s)", len(nodeList)))
		if !yes {
			return
		}
	}
	return apiNode.NodeDelete(&ndp)
}
