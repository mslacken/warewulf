package container

import (
	"fmt"
	"os"
	"strconv"

	"github.com/containers/image/v5/types"
	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/hpcng/warewulf/internal/pkg/container"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/util"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/pkg/errors"
)

func ContainerBuild(cbp *wwapiv1.ContainerBuildParameter) (err error) {

	if cbp == nil {
		return fmt.Errorf("ContainerBuildParameter is nil")
	}

	var containers []string

	if cbp.All {
		containers, err = container.ListSources()
	} else {
		containers = cbp.ContainerNames
	}

	if len(containers) == 0 {
		return
	}

	for _, c := range containers {
		if !container.ValidSource(c) {
			err = fmt.Errorf("VNFS name does not exist: %s", c)
			wwlog.Error("%s", err)
			return
		}

		err = container.Build(c, cbp.Force)
		if err != nil {
			wwlog.Error("Could not build container %s: %s", c, err)
			return
		}
	}

	if cbp.Default {
		if len(containers) != 1 {
			wwlog.Error("Can only set default for one container")
		} else {
			var nodeDB node.NodeYaml
			nodeDB, err = node.New()
			if err != nil {
				wwlog.Error("Could not open node configuration: %s", err)
				return
			}

			//TODO: Don't loop through profiles, instead have a nodeDB function that goes directly to the map
			profiles, _ := nodeDB.FindAllProfiles()
			for _, profile := range profiles {
				wwlog.Debug("Looking for profile default: %s", profile.Id.Get())
				if profile.Id.Get() == "default" {
					wwlog.Debug("Found profile default, setting container name to: %s", containers[0])
					profile.ContainerName.Set(containers[0])
					err := nodeDB.ProfileUpdate(profile)
					if err != nil {
						return errors.Wrap(err, "failed to update node profile")
					}
				}
			}
			// TODO: Need a wrapper and flock around this. Sometimes we restart warewulfd and sometimes we don't.
			err = nodeDB.Persist()
			if err != nil {
				return errors.Wrap(err, "failed to persist nodedb")
			}
			fmt.Printf("Set default profile to container: %s\n", containers[0])
		}
	}
	return
}

func ContainerDelete(cdp *wwapiv1.ContainerDeleteParameter) (err error) {

	if cdp == nil {
		return fmt.Errorf("ContainerDeleteParameter is nil")
	}

	nodeDB, err := node.New()
	if err != nil {
		wwlog.Error("Could not open nodeDB: %s", err)
		return
	}

	nodes, err := nodeDB.FindAllNodes()
	if err != nil {
		return
	}

ARG_LOOP:
	for i := 0; i < len(cdp.ContainerNames); i++ {
		//_, arg := range args {
		containerName := cdp.ContainerNames[i]
		for _, n := range nodes {
			if n.ContainerName.Get() == containerName {
				wwlog.Error("Container is configured for nodes, skipping: %s", containerName)
				continue ARG_LOOP
			}
		}

		if !container.ValidSource(containerName) {
			wwlog.Error("Container name is not a valid source: %s", containerName)
			continue
		}
		err := container.DeleteSource(containerName)
		if err != nil {
			wwlog.Error("Could not remove source: %s", containerName)
		}
		err = container.DeleteImage(containerName)
		if err != nil {
			wwlog.Error("Could not remove image files %s", containerName)
		}

		fmt.Printf("Container has been deleted: %s\n", containerName)
	}

	return
}

func ContainerList() (containerInfo []*wwapiv1.ContainerInfo, err error) {
	var sources []string

	sources, err = container.ListSources()
	if err != nil {
		wwlog.Error("%s", err)
		return
	}

	nodeDB, err := node.New()
	if err != nil {
		wwlog.Error("%s", err)
		return
	}

	nodes, err := nodeDB.FindAllNodes()
	if err != nil {
		wwlog.Error("%s", err)
		return
	}

	nodemap := make(map[string]int)
	for _, n := range nodes {
		nodemap[n.ContainerName.Get()]++
	}

	for _, source := range sources {
		if nodemap[source] == 0 {
			nodemap[source] = 0
		}

		wwlog.Debug("Finding kernel version for: %s", source)
		kernelVersion := container.KernelVersion(source)
		var creationTime uint64
		sourceStat, err := os.Stat(container.SourceDir(source))
		if err != nil {
			wwlog.Error("%s\n", err)
		} else {
			creationTime = uint64(sourceStat.ModTime().Unix())
		}
		var modTime uint64
		imageStat, err := os.Stat(container.ImageFile(source))
		if err != nil {
			wwlog.Error("%s\n", err)
		} else {
			modTime = uint64(imageStat.ModTime().Unix())
		}
		size, err := util.DirSize(container.SourceDir(source))
		if err != nil {
			wwlog.Error("%s\n", err)
		}
		imgSize, err := os.Stat(container.ImageFile(source))
		if err != nil {
			wwlog.Error("%s\n", err)
		} else {
			size += imgSize.Size()
		}
		imgSize, err = os.Stat(container.ImageFile(source) + ".gz")
		if err != nil {
			wwlog.Error("%s\n", err)
		} else {
			size += imgSize.Size()
		}

		containerInfo = append(containerInfo, &wwapiv1.ContainerInfo{
			Name:          source,
			NodeCount:     uint32(nodemap[source]),
			KernelVersion: kernelVersion,
			CreateDate:    creationTime,
			ModDate:       modTime,
			Size:          uint64(size),
		})

	}
	return
}

func ContainerShow(csp *wwapiv1.ContainerShowParameter) (response *wwapiv1.ContainerShowResponse, err error) {

	containerName := csp.ContainerName

	if !container.ValidName(containerName) {
		err = fmt.Errorf("%s is not a valid container", containerName)
		return
	}

	rootFsDir := container.RootFsDir(containerName)

	kernelVersion := container.KernelVersion(containerName)

	nodeDB, err := node.New()
	if err != nil {
		return
	}

	nodes, err := nodeDB.FindAllNodes()
	if err != nil {
		return
	}

	var nodeList []string
	for _, n := range nodes {
		if n.ContainerName.Get() == containerName {
			nodeList = append(nodeList, n.Id.Get())
		}
	}

	response = &wwapiv1.ContainerShowResponse{
		Name:          containerName,
		Rootfs:        rootFsDir,
		Nodes:         nodeList,
		KernelVersion: kernelVersion,
	}
	return
}

// Private helpers

func setOCICredentials(sCtx *types.SystemContext) error {
	username, userSet := os.LookupEnv("WAREWULF_OCI_USERNAME")
	password, passSet := os.LookupEnv("WAREWULF_OCI_PASSWORD")
	if userSet || passSet {
		if userSet && passSet {
			sCtx.DockerAuthConfig = &types.DockerAuthConfig{
				Username: username,
				Password: password,
			}
		} else {
			return fmt.Errorf("oci username and password env vars must be specified together")
		}
	}
	return nil
}

func setNoHTTPSOpts(sCtx *types.SystemContext) error {
	val, ok := os.LookupEnv("WAREWULF_OCI_NOHTTPS")
	if !ok {
		return nil
	}

	noHTTPS, err := strconv.ParseBool(val)
	if err != nil {
		return fmt.Errorf("while parsing insecure http option: %v", err)
	}

	// only set this if we want to disable, otherwise leave as undefined
	if noHTTPS {
		sCtx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(true)
	}
	sCtx.OCIInsecureSkipTLSVerify = noHTTPS

	return nil
}

func getSystemContext() (sCtx *types.SystemContext, err error) {
	sCtx = &types.SystemContext{}

	if err := setOCICredentials(sCtx); err != nil {
		return nil, err
	}

	if err := setNoHTTPSOpts(sCtx); err != nil {
		return nil, err
	}

	return sCtx, nil
}
