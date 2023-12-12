package apiprofile

import (
	"fmt"
	"os"

	apinode "github.com/hpcng/warewulf/internal/pkg/api/node"
	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/util"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// NodeSet is the wwapiv1 implmentation for updating nodeinfo fields.
func ProfileSet(set *wwapiv1.ProfileSetParameter) (err error) {
	if set == nil {
		return fmt.Errorf("NodeAddParameter is nil")
	}
	nodeDB, _, err := ProfileSetParameterCheck(set, false)
	if err != nil {
		return errors.Wrap(err, "Could not open database")
	}
	return apinode.DbSave(&nodeDB)
}

// ProfileSetParameterCheck does error checking on ProfileSetParameter.
// Output to the console if console is true.
// TODO: Determine if the console switch does wwlog or not.
// - console may end up being textOutput?
func ProfileSetParameterCheck(set *wwapiv1.ProfileSetParameter, console bool) (nodeDB node.NodeYaml, profileCount uint, err error) {
	if set == nil {
		err = fmt.Errorf("profile set parameter is nil")
		if console {
			fmt.Printf("%v\n", err)
			return
		}
	}

	if set.ProfileNames == nil {
		err = fmt.Errorf("profile set parameter: ProfileNames is nil")
		if console {
			fmt.Printf("%v\n", err)
			return
		}
	}

	nodeDB, err = node.New()
	if err != nil {
		wwlog.Error("Could not open configuration: %s", err)
		return
	}
	profiles := nodeDB.ListAllProfiles()
	// Note: This does not do expansion on the nodes.
	if set.AllProfiles || (len(set.ProfileNames) == 0) {
		if console {
			wwlog.Warn("this command will modify all profiles!")
		}
	}
	if len(profiles) == 0 {
		if console {
			wwlog.Warn("no profiles found")
		}
		return
	}
	var pConf node.NodeConf

	for _, p := range profiles {
		if util.InSlice(set.ProfileNames, p) {
			wwlog.Verbose("evaluating profile: %s", p)
			err = yaml.Unmarshal([]byte(set.NodeConfYaml), nodeDB.NodeProfiles[p])
			if set.NetdevDelete != "" {
				if _, ok := nodeDB.NodeProfiles[p].NetDevs[set.NetdevDelete]; !ok {
					err = fmt.Errorf("network device name doesn't exist: %s", set.NetdevDelete)
					wwlog.Error(fmt.Sprintf("%v", err.Error()))
					return
				}
				wwlog.Verbose("Profile: %s, Deleting network device: %s", p, set.NetdevDelete)
				delete(nodeDB.NodeProfiles[p].NetDevs, set.NetdevDelete)
			}
			if set.PartitionDelete != "" {
				deletedPart := false
				for diskname, disk := range nodeDB.NodeProfiles[p].Disks {
					if _, ok := disk.Partitions[set.PartitionDelete]; ok {
						wwlog.Verbose("Node: %s, on disk %, deleting partition: %s", p, diskname, set.PartitionDelete)
						deletedPart = true
						delete(disk.Partitions, set.PartitionDelete)
					}
					if !deletedPart {
						wwlog.Error(fmt.Sprintf("%v", err.Error()))
						err = fmt.Errorf("partition doesn't exist: %s", set.PartitionDelete)
						return
					}
				}
			}
			if set.DiskDelete != "" {
				if _, ok := nodeDB.NodeProfiles[p].Disks[set.DiskDelete]; !ok {
					err = fmt.Errorf("disk doesn't exist: %s", set.DiskDelete)
					wwlog.Error(fmt.Sprintf("%v", err.Error()))
					return
				}
				wwlog.Verbose("Node: %s, deleting disk: %s", p, set.DiskDelete)
				delete(nodeDB.NodeProfiles[p].Disks, set.DiskDelete)
			}
			if set.FilesystemDelete != "" {
				if _, ok := nodeDB.NodeProfiles[p].FileSystems[set.FilesystemDelete]; !ok {
					err = fmt.Errorf("disk doesn't exist: %s", set.FilesystemDelete)
					wwlog.Error(fmt.Sprintf("%v", err.Error()))
					return
				}
				wwlog.Verbose("Node: %s, deleting filesystem: %s", p, set.FilesystemDelete)
				delete(nodeDB.NodeProfiles[p].FileSystems, set.FilesystemDelete)
			}

			for _, key := range pConf.TagsDel {
				delete(nodeDB.NodeProfiles[p].Tags, key)
			}
			for _, key := range pConf.Ipmi.TagsDel {
				delete(nodeDB.NodeProfiles[p].Ipmi.Tags, key)
			}
			for net := range pConf.NetDevs {
				for _, key := range pConf.NetDevs[net].TagsDel {
					if _, ok := nodeDB.NodeProfiles[p].NetDevs[net]; ok {
						delete(nodeDB.NodeProfiles[p].NetDevs[net].Tags, key)
					}
				}
			}
			if err != nil {
				wwlog.Error("%s", err)
				os.Exit(1)
			}
			profileCount++
		}
	}
	return
}

/*
Adds a new profile with the given name
*/
func AddProfile(nsp *wwapiv1.ProfileSetParameter) error {
	if nsp == nil {
		return fmt.Errorf("NodeSetParameter is nill")
	}
	nodeDB, err := node.New()
	if err != nil {
		return errors.Wrap(err, "Could not open database")
	}

	if util.InSlice(nodeDB.ListAllProfiles(), nsp.ProfileNames[0]) {
		return errors.New(fmt.Sprintf("profile with name %s already exists", nsp.ProfileNames[0]))
	}

	p, err := nodeDB.AddProfile(nsp.ProfileNames[0])
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
