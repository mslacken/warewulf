package container

import (
	"fmt"
	"os"
	"path"

	"github.com/hpcng/warewulf/internal/pkg/container"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
)

/*
Removes stray/unwanted directories from a container, e.g. like a
.snapshots directory
*/
func RemoveDir(name string, paths []string) error {
	if !container.ValidSource(name) {
		return fmt.Errorf("%s is not a valid container", name)
	}
	for _, pth := range paths {
		_, err := os.Stat(path.Join(container.RootFsDir(name), pth))
		if err != nil {
			wwlog.Verbose("could not stat %s: %s", pth, err)
			continue
		}
		wwlog.Verbose("deleting %s in %s", pth, container.RootFsDir(name))
		os.RemoveAll(path.Join(container.RootFsDir(name), pth))
	}
	return nil
}
