//go:build linux
// +build linux

package shell

import (
	"os"

	cntexec "github.com/hpcng/warewulf/internal/app/wwctl/container/exec"
	"github.com/hpcng/warewulf/internal/pkg/container"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/spf13/cobra"
)

func CobraRunE(cmd *cobra.Command, args []string) error {

	containerName := args[0]
	var allargs []string

	if !container.ValidSource(containerName) {
		wwlog.Error("Unknown Warewulf container: %s", containerName)
		os.Exit(1)
	}

	for _, b := range binds {
		allargs = append(allargs, "--bind", b)
	}
	allargs = append(allargs, args...)
	allargs = append(allargs, "/usr/bin/bash")
	return cntexec.CobraRunE(cmd, allargs)
}
