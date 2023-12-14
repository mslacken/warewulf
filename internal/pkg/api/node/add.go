package apinode

import (
	"encoding/hex"
	"fmt"

	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/util"
	"github.com/hpcng/warewulf/internal/pkg/warewulfd"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/hpcng/warewulf/pkg/hostlist"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// NodeAdd adds nodes for management by Warewulf.
func NodeAdd(nap *wwapiv1.NodeAddParameter) (err error) {

	if nap == nil {
		return fmt.Errorf("NodeAddParameter is nil")
	}

	nodeDB, err := node.New()
	if err != nil {
		return errors.Wrap(err, "failed to open node database")
	}
	dbHash := nodeDB.Hash()
	if hex.EncodeToString(dbHash[:]) != nap.Hash && !nap.Force {
		return fmt.Errorf("got wrong hash, not modifying node database")
	}
	node_args := hostlist.Expand(nap.NodeNames)
	var ipaddr, ipmiaddr string
	for _, a := range node_args {
		n, err := nodeDB.AddNode(a)
		if err != nil {
			return errors.Wrap(err, "failed to add node")
		}
		err = yaml.Unmarshal([]byte(nap.NodeConfYaml), &n)
		if err != nil {
			return errors.Wrap(err, "Failed to decode nodeConf")
		}
		wwlog.Info("Added node: %s", a)
		var netName string
		// sets netname to the only key of n.NetDevs
		for netName = range n.NetDevs {
		}
		if netName != "" {
			if ipaddr != "" {
				// if more nodes are added increment IPv4 address
				ipaddr = util.IncrementIPv4(ipaddr, 1)
				wwlog.Verbose("Incremented IP addr to %s", ipaddr)
				n.NetDevs[netName].Ipaddr = ipaddr

			} else {
				ipaddr = n.NetDevs[netName].Ipaddr
			}
		}
		if n.Ipmi != nil {
			if ipmiaddr != "" {
				// if more nodes are added increment IPv4 address
				ipmiaddr = util.IncrementIPv4(ipmiaddr, 1)
				wwlog.Verbose("Incremented IP addr to %s", ipmiaddr)
				n.Ipmi.Ipaddr = ipmiaddr
			} else {
				ipmiaddr = n.Ipmi.Ipaddr
			}
		}

	}

	err = nodeDB.Persist()
	if err != nil {
		return errors.Wrap(err, "failed to persist new node")
	}

	err = warewulfd.DaemonReload()
	if err != nil {
		return errors.Wrap(err, "failed to reload warewulf daemon")
	}
	return
}
