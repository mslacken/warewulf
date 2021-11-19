package buildhybrid

import (
	"os"
	"regexp"

	"github.com/hpcng/warewulf/internal/pkg/hybridcontainer"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/spf13/cobra"
)

func CobraRunE(cmd *cobra.Command, args []string) error {
	basecontainer := args[0]
	if match, _ := regexp.MatchString(".*-suffix$", basecontainer); match {
		wwlog.Printf(wwlog.ERROR, "Can't build hybrid container from a hybrid existing the hybrid container '%s'\n", basecontainer)
		os.Exit(1)
	}
	err := hybridcontainer.Build(basecontainer, AdditionalExec)
	if err != nil {
		wwlog.Printf(wwlog.ERROR, "%s\n", err)
		os.Exit(1)
	}
	return nil
}
