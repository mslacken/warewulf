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
	var confobject node.NodeConf
	for _, p := range confs {
		if util.InSlice(set.ConfList, p) {
			wwlog.Verbose("evaluating profile: %s", p)
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
				deletedPart := false
				for diskname, disk := range confMap[p].Disks {
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
				if _, ok := confMap[p].Disks[set.DiskDelete]; !ok {
					err = fmt.Errorf("disk doesn't exist: %s", set.DiskDelete)
					wwlog.Error(fmt.Sprintf("%v", err.Error()))
					return
				}
				wwlog.Verbose("Node: %s, deleting disk: %s", p, set.DiskDelete)
				delete(confMap[p].Disks, set.DiskDelete)
			}
			if set.FilesystemDelete != "" {
				if _, ok := confMap[p].FileSystems[set.FilesystemDelete]; !ok {
					err = fmt.Errorf("disk doesn't exist: %s", set.FilesystemDelete)
					wwlog.Error(fmt.Sprintf("%v", err.Error()))
					return
				}
				wwlog.Verbose("Node: %s, deleting filesystem: %s", p, set.FilesystemDelete)
				delete(confMap[p].FileSystems, set.FilesystemDelete)
			}

			for _, key := range confobject.TagsDel {
				delete(confMap[p].Tags, key)
			}
			for _, key := range confobject.Ipmi.TagsDel {
				delete(confMap[p].Ipmi.Tags, key)
			}
			for net := range confobject.NetDevs {
				for _, key := range confobject.NetDevs[net].TagsDel {
					if _, ok := confMap[p].NetDevs[net]; ok {
						delete(confMap[p].NetDevs[net].Tags, key)
					}
				}
			}
			if err != nil {
				return
			}
			count++
		}
	}
	return
}
