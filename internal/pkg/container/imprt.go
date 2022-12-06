package container

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

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
func ImportImage(uri string, name string, cpyOutPth string) error {
	fullPath := RootFsDir(name)
	err := os.MkdirAll(fullPath, 0755)
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
