package apinode

import (
	"fmt"

	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/util"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"gopkg.in/yaml.v2"

	"github.com/hpcng/warewulf/internal/pkg/warewulfd"
)

// NodeSet is the wwapiv1 implmentation for updating node fields.
func NodeSet(set *wwapiv1.ConfSetParameter) (err error) {
	if set == nil {
		return fmt.Errorf("NodeSetParameter is nil")
	}
	var nodeDB node.NodeYaml
	nodeDB, _, err = NodeSetParameterCheck(set)
	if err != nil {
		return err
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
func NodeSetParameterCheck(set *wwapiv1.ConfSetParameter) (nodeDB node.NodeYaml, count uint, err error) {
	nodeDB, err = node.New()
	if err != nil {
		wwlog.Error("Could not open configuration: %s", err)
		return
	}
	nodes := nodeDB.ListAllNodes()
	count, err = AbstractSetParameterCheck(set, nodeDB.Nodes, nodes)
	return nodeDB, count, err
}

func AbstractSetParameterCheck(set *wwapiv1.ConfSetParameter, confMap map[string]*node.NodeConf, confs []string) (count uint, err error) {
	if set == nil {
		err = fmt.Errorf("profile set parameter is nil")
		return
	}
	if set.ConfList == nil {
		err = fmt.Errorf("profile set parameter: ConfListis nil")
		return
	}
	// Note: This does not do expansion on the nodes.
	if set.AllConfs || (len(set.ConfList) == 0) {
		wwlog.Warn("this command will modify all nodes/profiles")
	} else if len(confs) == 0 {
		wwlog.Warn("no nodes/profiles found")
		return
	} else {
		confs = set.ConfList
	}
	//var confobject node.NodeConf
	for _, p := range confs {
		if util.InSlice(set.ConfList, p) {
			wwlog.Verbose("evaluating profile: %s", p)
			if _, ok := confMap[p]; !ok {
				continue
			}
			err = yaml.Unmarshal([]byte(set.NodeConfYaml), confMap[p])
			if err != nil {
				return
			}
			if set.NetdevDelete != "" {
				if _, ok := confMap[p].NetDevs[set.NetdevDelete]; !ok {
					err = fmt.Errorf("network device name doesn't exist: %s", set.NetdevDelete)
					wwlog.Error(fmt.Sprintf("%v", err.Error()))
					return
				}
				wwlog.Verbose("Profile: %s, Deleting network device: %s", p, set.NetdevDelete)
				delete(confMap[p].NetDevs, set.NetdevDelete)
			}
			if set.PartitionDelete != "" {
				for diskname, disk := range confMap[p].Disks {
					if _, ok := disk.Partitions[set.PartitionDelete]; ok {
						wwlog.Verbose("Node: %s, on disk %, deleting partition: %s", p, diskname, set.PartitionDelete)
						delete(disk.Partitions, set.PartitionDelete)
					} else {
						return count, fmt.Errorf("partition doesn't exist: %s", set.PartitionDelete)

					}
				}
			}
			if set.DiskDelete != "" {
				if _, ok := confMap[p].Disks[set.DiskDelete]; ok {
					wwlog.Verbose("Node: %s, deleting disk: %s", p, set.DiskDelete)
					delete(confMap[p].Disks, set.DiskDelete)
				} else {
					return count, fmt.Errorf("disk doesn't exist: %s", set.DiskDelete)
				}
			}
			if set.FilesystemDelete != "" {
				if _, ok := confMap[p].FileSystems[set.FilesystemDelete]; ok {
					wwlog.Verbose("Node: %s, deleting filesystem: %s", p, set.FilesystemDelete)
					delete(confMap[p].FileSystems, set.FilesystemDelete)
				} else {
					return count, fmt.Errorf("disk doesn't exist: %s", set.FilesystemDelete)
				}
			}
			for _, key := range set.TagDel {
				delete(confMap[p].Tags, key)
			}
			for key, val := range set.TagAdd {
				confMap[p].Tags[key] = val
			}
			for key, val := range set.IpmiTagAdd {
				if confMap[p].Ipmi.Tags == nil {
					confMap[p].Ipmi.Tags = make(map[string]string)
				}
				confMap[p].Ipmi.Tags[key] = val
			}
			for _, key := range set.IpmiTagDel {
				delete(confMap[p].Ipmi.Tags, key)
			}
			if _, ok := confMap[p].NetDevs[set.Netdev]; ok {
				for _, key := range set.NetTagDel {
					delete(confMap[p].NetDevs[set.Netdev].Tags, key)
				}
				for key, val := range set.TagAdd {
					confMap[p].NetDevs[set.Netdev].Tags[key] = val
				}
			}
			count++
		}
	}
	return
}
