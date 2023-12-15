package add

import (
	"fmt"
	"strings"

	apiprofile "github.com/hpcng/warewulf/internal/pkg/api/profile"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"gopkg.in/yaml.v2"

	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/spf13/cobra"
)

func CobraRunE(vars *variables) func(cmd *cobra.Command, args []string) (err error) {
	return func(cmd *cobra.Command, args []string) (err error) {
		// run converters for different types
		for _, c := range vars.converters {
			if err := c(); err != nil {
				return err
			}
		}
		// remove the UNDEF network as all network values are assigned
		// to this network
		if !node.ObjectIsEmpty(vars.profileConf.NetDevs["UNDEF"]) {
			netDev := *vars.profileConf.NetDevs["UNDEF"]
			vars.profileConf.NetDevs[vars.netName] = &netDev
		}
		delete(vars.profileConf.NetDevs, "UNDEF")
		if vars.fsName != "" {
			if !strings.HasPrefix(vars.fsName, "/dev") {
				if vars.fsName == vars.partName {
					vars.fsName = "/dev/disk/by-partlabel/" + vars.partName
				} else {
					return fmt.Errorf("filesystems need to have a underlying blockdev")
				}
			}
			fs := *vars.profileConf.FileSystems["UNDEF"]
			vars.profileConf.FileSystems[vars.fsName] = &fs
		}
		delete(vars.profileConf.FileSystems, "UNDEF")
		if vars.diskName != "" && vars.partName != "" {
			prt := *vars.profileConf.Disks["UNDEF"].Partitions["UNDEF"]
			vars.profileConf.Disks["UNDEF"].Partitions[vars.partName] = &prt
			delete(vars.profileConf.Disks["UNDEF"].Partitions, "UNDEF")
			dsk := *vars.profileConf.Disks["UNDEF"]
			vars.profileConf.Disks[vars.diskName] = &dsk
		}
		if (vars.diskName != "") != (vars.partName != "") {
			return fmt.Errorf("partition and disk must be specified")
		}
		delete(vars.profileConf.Disks, "UNDEF")
		buffer, err := yaml.Marshal(vars.profileConf)
		if err != nil {
			wwlog.Error("Can't marshall nodeInfo", err)
			return err
		}
		set := wwapiv1.NodeAddParameter{
			NodeConfYaml: string(buffer[:]),
			NodeNames:    args,
			Force:        true,
		}
		return apiprofile.ProfileAdd(&set)
	}
}
