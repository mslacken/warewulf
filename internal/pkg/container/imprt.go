package container

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"libguestfs.org/guestfs"

	"github.com/containers/image/v5/types"
	"github.com/containers/storage/drivers/copy"
	"github.com/pkg/errors"

	"github.com/hpcng/warewulf/internal/pkg/oci"
	"github.com/hpcng/warewulf/internal/pkg/util"
	"github.com/hpcng/warewulf/internal/pkg/warewulfconf"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
)

/*
Import a docker container to warewulf with the given name
*/
func ImportDocker(uri string, name string, sCtx *types.SystemContext) error {
	OciBlobCacheDir := warewulfconf.DataStore() + "/oci"

	err := os.MkdirAll(OciBlobCacheDir, 0755)
	if err != nil {
		return err
	}

	if !ValidName(name) {
		return errors.New("VNFS name contains illegal characters: " + name)
	}

	fullPath := RootFsDir(name)

	err = os.MkdirAll(fullPath, 0755)
	if err != nil {
		return err
	}

	p, err := oci.NewPuller(
		oci.OptSetBlobCachePath(OciBlobCacheDir),
		oci.OptSetSystemContext(sCtx),
	)
	if err != nil {
		return err
	}

	if _, err := p.GenerateID(context.Background(), uri); err != nil {
		return err
	}

	if err := p.Pull(context.Background(), uri, fullPath); err != nil {
		return err
	}

	return nil
}

/*
Import a directory as container with the given name.
*/
func ImportDirectory(uri string, name string) error {
	fullPath := RootFsDir(name)
	err := os.MkdirAll(fullPath, 0755)
	if err != nil {
		return err
	}
	if !util.IsDir(uri) {
		return errors.New("Import directory does not exist: " + uri)
	}
	if !util.IsFile(path.Join(uri, "/bin/sh")) {
		return errors.New("Source directory has no /bin/sh: " + uri)
	}
	err = copy.DirCopy(uri, fullPath, copy.Content, true)
	if err != nil {
		return err
	}
	return nil
}

/*
Import a image as container with the given name
*/
func ImportImageGuestFish(uri string, name string, cpyOutPth string) error {
	cpyOutPth, err := exec.LookPath("guestfish")
	if err != nil {
		return fmt.Errorf("could not locate 'virt-copy-out' needed for image import (install libguestfs): %s", err)
	}
	fullPath := RootFsDir(name)
	err = os.MkdirAll(fullPath, 0755)
	if err != nil {
		return err
	}
	args := []string{"--ro", "-i", "copy-out", "-a", uri, "/", fullPath}
	wwlog.Verbose("Trying to run %s", string(cpyOutPth+" "+strings.Join(args, " ")))
	proc := exec.Command(cpyOutPth, args...)
	out, err := proc.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("couldn't run %s:%s\nOuput:%s", cpyOutPth, err, out)
		return err
	}
	return err
}

/*
Import a image with GOC bindings of libc
*/
func ImportImage(uri string, name string) error {
	gh, err := guestfs.Create()
	if err != nil {
		return fmt.Errorf("could not create binding to libguestfs: %s", err)
	}
	optargs := guestfs.OptargsAdd_drive{
		Format_is_set:   true,
		Format:          "raw",
		Readonly_is_set: true,
		Readonly:        true,
	}
	if err := gh.Add_drive(uri, &optargs); err != nil {
		return fmt.Errorf("Could not add image %s as root drive: %s", uri, err)
	}

	/* Run the libguestfs back-end. */
	if err := gh.Launch(); err != nil {
		panic(err)
	}
	if err != nil {
		return fmt.Errorf("could not create handle to libguestfs: %s", err)
	}
	// serach root drives in the image
	roots, err := gh.Inspect_os()
	if err != nil {
		return fmt.Errorf("error at os detection: %s", err)
	}
	if len(roots) == 0 {
		return fmt.Errorf("function ImportImage: no operating systems found")
	}
	return nil
}
