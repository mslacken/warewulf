package build

import (
	"errors"
	"os"
	"strings"

	warewulfconf "github.com/hpcng/warewulf/internal/pkg/config"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/overlay"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/hpcng/warewulf/pkg/hostlist"
	"github.com/spf13/cobra"
)

func CobraRunE(cmd *cobra.Command, args []string) error {
	controller := warewulfconf.Get()
	nodeDB, err := node.New()
	if err != nil {
		wwlog.Error("Could not open node configuration: %s", err)
		os.Exit(1)
	}

	db, err := nodeDB.FindAllNodes()
	if err != nil {
		wwlog.Error("Could not get node list: %s", err)
		os.Exit(1)
	}

	if len(args) > 0 {
		args = hostlist.Expand(args)
		db = node.FilterByName(db, args)

		if len(db) < len(args) {
			return errors.New("Failed to find nodes")
		}
	}

	// NOTE: this is to keep backward compatible
	// passing -O a,b,c versus -O a -O b -O c, but will also accept -O a,b -O c
	overlayNames := []string{}
	for _, name := range OverlayNames {
		names := strings.Split(name, ",")
		overlayNames = append(overlayNames, names...)
	}
	OverlayNames = overlayNames

	if OverlayDir != "" {
		if len(OverlayNames) == 0 {
			// TODO: should this behave the same as OverlayDir == "", and build default
			// set to overlays?
			return errors.New("must specify overlay(s) to build")
		}

		if len(args) > 0 {
			if len(db) != 1 {
				return errors.New("nust specify one node to build overlay")
			}

			for _, node := range db {
				return overlay.BuildOverlayIndir(node, OverlayNames, OverlayDir)
			}
		} else {
			// TODO this seems different than what is set in BuildHostOverlay
			hostname, _ := os.Hostname()
			node := node.NewConf(hostname)
			wwlog.Info("building overlay for host: %s", hostname)
			return overlay.BuildOverlayIndir(node, OverlayNames, OverlayDir)

		}

	}

	if BuildHost && controller.Warewulf.EnableHostOverlay {
		err := overlay.BuildHostOverlay()
		if err != nil {
			wwlog.Warn("host overlay could not be built: %s", err)
		}
	}

	if BuildNodes || (!BuildHost && !BuildNodes) {
		if len(OverlayNames) > 0 {
			err = overlay.BuildSpecificOverlays(db, OverlayNames)
		} else {
			err = overlay.BuildAllOverlays(db)
		}

		if err != nil {
			wwlog.Warn("Some overlays failed to be generated: %s", err)
		}
	}
	return nil
}
