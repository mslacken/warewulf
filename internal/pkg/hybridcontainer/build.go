package hybridcontainer

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/hpcng/warewulf/internal/pkg/config"
	"github.com/hpcng/warewulf/internal/pkg/container"
	"github.com/hpcng/warewulf/internal/pkg/util"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/pkg/errors"
)

var tempLocation = "/var/tmp"

func Build(name string, additionExec []string) error {
	var PATH = []string{"/bin", "/sbin", "/usr/bin/", "/usr/sbin/", "/usr/local/bin", "/usr/local/sbin"}
	var neededExecs = []string{"casync", "dracut", "mkfs.ext3"}
	var dracutModules = "/usr/lib/dracut/modules.d/50ww-hybrid"
	wwlog.Printf(wwlog.DEBUG, "Starting to build hybrid container "+name)
	if !container.ValidSource(name) {
		return errors.New("Base container does not exist")
	}

	// find out if all needed executeables are in the container
	rootfsPath := container.RootFsDir(name)
	// copy
	_, err := os.Stat(path.Join(rootfsPath, dracutModules))
	if os.IsNotExist(err) {
		util.CopyFiles(dracutModules, rootfsPath)
	}
	var allFilesInPath []string
	for _, myPath := range PATH {
		files, _ := ioutil.ReadDir(path.Join(rootfsPath, myPath))
		for _, file := range files {
			allFilesInPath = append(allFilesInPath, file.Name())
		}
	}
	var notFound []string
	for _, exec := range append(neededExecs, additionExec...) {
		var foundExec bool = false
		for _, file := range allFilesInPath {
			if file == exec {
				wwlog.Printf(wwlog.DEBUG, "Found "+exec)
				foundExec = true
			}
		}
		if !foundExec {
			notFound = append(notFound, exec)
		}
	}
	if len(notFound) != 0 {
		return errors.New(fmt.Sprintf("Did not find %v in container %s", notFound, name))
	}
	// now get the dracut executeable
	var dracutExec string
	for _, myPath := range PATH {
		files, _ := ioutil.ReadDir(path.Join(rootfsPath, myPath))
		for _, file := range files {
			if file.Name() == "dracut" {
				dracutExec = path.Join(myPath, "dracut")
				wwlog.Printf(wwlog.DEBUG, "Using "+dracutExec+" for executing dracut")

			}
		}
	}

	// call dracut with the right options in the container, the resulting ramdisk will
	// be on the bind mount
	// TODO: do not call wwctl but use a direct API call
	// commandline option can't be set for baarcmd with wwctl container exec foocontainer baarcmd
	// due to cobra https://github.com/spf13/cobra/issues/739
	// so we create a hand crafted dracut shell script with the right options
	var dracutCmdName = tempLocation + "/ww-dracut.cmd." + fmt.Sprintf("%v", os.Getpid())
	var archiveName = tempLocation + "/" + name + "-hybrid.cpio." + fmt.Sprintf("%v", os.Getpid())
	dracutCmdFile, err := os.Create(dracutCmdName)
	if err != nil {
		return err
	}
	defer dracutCmdFile.Close()
	err = os.Chmod(dracutCmdName, 0700)
	dracutCmdFile.WriteString("#!/bin/bash\n")
	var dracutCmdOpts = dracutExec + `\
		--no-kernel\
		--no-compress\
		--no-early-microcode\
		--no-hostonly-cmdline\
		--modules 'base ww-hybrid'\
		--install 'chmod stat casync /usr/lib/casync/protocols/casync-http ls find tr uniq'\
		` + archiveName
	dracutCmdFile.WriteString(dracutCmdOpts + "\n")
	var wwctlArgs = []string{"container", "exec", "--bind", tempLocation, name, "/bin/bash", dracutCmdName}
	wwlog.Printf(wwlog.INFO, "Starting dracut in container with: wwctl %v", wwctlArgs)
	wwlog.Printf(wwlog.INFO, "dracut command is %s", dracutCmdOpts)
	cmdDracut := exec.Command("wwctl", wwctlArgs...)
	cmdDracut.Stdout = os.Stdout
	cmdDracut.Stderr = os.Stderr
	err = cmdDracut.Run()
	if err != nil {
		return errors.Wrap(err, "Error in running dracut command in container")
	}
	dracutCmdFile.Close()
	os.Remove(dracutCmdName)
	// now unpack the created image and rename init to init.initrd (I know shelling out is evil but cpio wants a pipe)
	archiveDir := archiveName + ".dir"
	compressor, err := exec.LookPath("pigz")
	if err != nil {
		wwlog.Printf(wwlog.VERBOSE, "Could not locate PIGZ, using GZIP\n")
		compressor = "gzip"
	} else {
		wwlog.Printf(wwlog.VERBOSE, "Using PIGZ to recompress the container: %s\n")
	}
	wwlog.Printf(wwlog.DEBUG, "Rebuilding hybrid image: '%s'")
	cmdCpio := fmt.Sprintf(`cpio -id -D %s < %s && cd %s && mv -v init init.initrd && find . | cpio --quiet -o -H newc | %s -c > %s`,
		archiveDir, archiveName, archiveDir, compressor, container.ImageParentDir()+"/"+name+"-hybrid.img.gz")

	wwlog.Printf(wwlog.DEBUG, "RUNNING: %s\n", cmdCpio)
	err = exec.Command("/bin/sh", "-c", cmdCpio).Run()
	if err != nil {
		return errors.Wrap(err, "Rebuilding of the hybrid container failed")
	}
	err = os.Remove(archiveName)
	if err != nil {
		return err
	}
	err = os.RemoveAll(archiveDir)
	if err != nil {
		return err
	}

	// running now casync
	if !util.IsDir(config.CasyncDir()) {
		err = os.MkdirAll(config.CasyncDir(), 0755)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failed to create casyncdir '%s'", config.CasyncDir()))
		}
	}
	cmdCasyncStr := fmt.Sprintf(`casync make \
		--with=unix\
		--without=flag-nocow\
		--without=flag-immutable\
		--without=subvolume-ro\
		--without=subvolume \
		%s.caidx %s`, path.Join(config.CasyncDir(), name), rootfsPath)
	cmdCasync := exec.Command("/bin/sh", "-c", cmdCasyncStr)
	cmdCasync.Stdout = os.Stdout
	cmdCasync.Stderr = os.Stderr
	err = cmdCasync.Run()
	if err != nil {
		return errors.Wrap(err, "Running casync failed")
	}
	return nil
}
