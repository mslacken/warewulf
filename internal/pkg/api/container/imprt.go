package container

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/hpcng/warewulf/internal/pkg/container"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/util"
	"github.com/hpcng/warewulf/internal/pkg/warewulfd"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/pkg/errors"
)

/*
Import a container form the given source. If Name is empty the last noun of
source is used as name. Supported formats are osc containers and chroot directories .
*/
func ContainerImport(cip *wwapiv1.ContainerImportParameter) (containerName string, err error) {

	if cip == nil {
		err = fmt.Errorf("NodeAddParameter is nil")
		return
	}

	if cip.Name == "" {
		name := path.Base(cip.Source)
		wwlog.Info("Setting VNFS name: %s", name)
		cip.Name = name
	}
	if !container.ValidName(cip.Name) {
		err = fmt.Errorf("VNFS name contains illegal characters: %s", cip.Name)
		wwlog.Error(err.Error())
		return
	}

	containerName = cip.Name
	fullPath := container.SourceDir(cip.Name)

	if util.IsDir(fullPath) {
		if cip.Force {
			wwlog.Info("Overwriting existing VNFS")
			err = os.RemoveAll(fullPath)
			if err != nil {
				wwlog.ErrorExc(err, "")
				return
			}
		} else if cip.Update {
			wwlog.Info("Updating existing VNFS")
		} else {
			err = fmt.Errorf("VNFS Name exists, specify --force, --update, or choose a different name: %s", cip.Name)
			wwlog.Error(err.Error())
			return
		}
	} else if strings.HasPrefix(cip.Source, "docker://") || strings.HasPrefix(cip.Source, "docker-daemon://") ||
		(strings.HasPrefix(cip.Source, "file://") &&
			(strings.HasSuffix(cip.Source, "tar") || strings.HasSuffix(cip.Source, ".tar.gz"))) || util.IsFile(cip.Source) {
		var sCtx *types.SystemContext
		sCtx, err = getSystemContext()
		if err != nil {
			wwlog.ErrorExc(err, "")
			// TODO: mhink - return was missing here. Was that deliberate?
		}

		err = container.ImportDocker(cip.Source, cip.Name, sCtx)
		if err != nil {
			err = fmt.Errorf("could not import image: %s", err.Error())
			wwlog.Error(err.Error())
			_ = container.DeleteSource(cip.Name)
			return
		}
	} else if strings.HasPrefix(cip.Source, "qcow://") || strings.HasPrefix(cip.Source, "raw://") || strings.HasPrefix(cip.Source, "image://") {
		realPath := cip.Source
		for _, prefix := range []string{"qcow://", "image://", "raw://"} {
			realPath = strings.TrimPrefix(realPath, prefix)
		}
		fmt.Println("realPath", realPath)
		var cpyOut string
		cpyOut, err = exec.LookPath("guestfish")
		if err != nil {
			wwlog.ErrorExc(err, "could not locate 'virt-copy-out' needed for image import (install libguestfs)")
			return
		}
		err = container.ImportImage(realPath, cip.Name, cpyOut)
		if err != nil {
			err = fmt.Errorf("could not import image: %s", err.Error())
			wwlog.Error(err.Error())
			_ = container.DeleteSource(cip.Name)
			return
		}
	} else if util.IsDir(cip.Source) {
		err = container.ImportDirectory(cip.Source, cip.Name)
		if err != nil {
			err = fmt.Errorf("could not import image: %s", err.Error())
			wwlog.Error(err.Error())
			_ = container.DeleteSource(cip.Name)
			return
		}
	} else {
		err = fmt.Errorf("invalid dir or uri: %s", cip.Source)
		wwlog.Error(err.Error())
		return
	}

	wwlog.Info("Updating the container's /etc/resolv.conf")
	err = util.CopyFile("/etc/resolv.conf", path.Join(container.RootFsDir(cip.Name), "/etc/resolv.conf"))
	if err != nil {
		wwlog.Warn("Could not copy /etc/resolv.conf into container: %s", err)
	}

	err = container.SyncUids(cip.Name, !cip.SyncUser)
	if err != nil && !cip.SyncUser {
		err = fmt.Errorf("error in user sync, fix error and run 'syncuser' manually: %s", err)
		wwlog.Error(err.Error())
		return
	}

	wwlog.Info("Building container: %s", cip.Name)
	err = container.Build(cip.Name, true)
	if err != nil {
		err = fmt.Errorf("could not build container %s: %s", cip.Name, err.Error())
		wwlog.Error(err.Error())
		return
	}

	if cip.Default {
		var nodeDB node.NodeYaml
		nodeDB, err = node.New()
		if err != nil {
			err = fmt.Errorf("could not open node configuration: %s", err.Error())
			wwlog.Error(err.Error())
			return
		}

		//TODO: Don't loop through profiles, instead have a nodeDB function that goes directly to the map
		profiles, _ := nodeDB.FindAllProfiles()
		for _, profile := range profiles {
			wwlog.Debug("Looking for profile default: %s", profile.Id.Get())
			if profile.Id.Get() == "default" {
				wwlog.Debug("Found profile default, setting container name to: %s", cip.Name)
				profile.ContainerName.Set(cip.Name)
				err = nodeDB.ProfileUpdate(profile)
				if err != nil {
					err = errors.Wrap(err, "failed to update profile")
					return
				}
			}
		}
		// TODO: We need this in a function with a flock around it.
		// Also need to understand if the daemon restart is only to
		// reload the config or if there is something more.
		err = nodeDB.Persist()
		if err != nil {
			err = errors.Wrap(err, "failed to persist nodedb")
			return
		}

		wwlog.Info("Set default profile to container: %s", cip.Name)
		err = warewulfd.DaemonReload()
		if err != nil {
			err = errors.Wrap(err, "failed to reload warewulf daemon")
			return
		}
	}
	return
}
