//go:build linux
// +build linux

package child

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hpcng/warewulf/internal/pkg/container"
	"github.com/hpcng/warewulf/internal/pkg/util"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func CobraRunE(cmd *cobra.Command, args []string) error {
	if os.Getpid() != 1 {
		wwlog.Error("PID is not 1: %d", os.Getpid())
		os.Exit(1)
	}

	containerName := args[0]

	if !container.ValidSource(containerName) {
		wwlog.Error("Unknown Warewulf container: %s", containerName)
		os.Exit(1)
	}
	containerPath := container.RootFsDir(containerName)
	fileStat, _ := os.Stat(path.Join(containerPath, "/etc/passwd"))
	unixStat := fileStat.Sys().(*syscall.Stat_t)
	passwdTime := time.Unix(int64(unixStat.Ctim.Sec), int64(unixStat.Ctim.Nsec))
	fileStat, _ = os.Stat(path.Join(containerPath, "/etc/group"))
	unixStat = fileStat.Sys().(*syscall.Stat_t)
	groupTime := time.Unix(int64(unixStat.Ctim.Sec), int64(unixStat.Ctim.Nsec))
	wwlog.Debug("passwd: %v", passwdTime)
	wwlog.Debug("group: %v", groupTime)

	err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	if err != nil {
		return errors.Wrap(err, "failed to mount")
	}

	err = syscall.Mount("/dev", path.Join(containerPath, "/dev"), "", syscall.MS_BIND, "")
	if err != nil {
		return errors.Wrap(err, "failed to mount /dev")
	}
	var createdFileObjs []string
	for _, b := range binds {
		fileObsIndx := len(createdFileObjs)
		var source string
		var dest string

		bind := util.SplitValidPaths(b, ":")
		source = bind[0]

		if len(bind) == 1 {
			dest = source
		} else {
			dest = bind[1]
		}

		if !util.IsFile(path.Join(containerPath, dest)) {
			/*
				foo := func(baar string) {
					fmt.Println(createdFileObjs, baar)
				}
				foo("foobaar")
			*/
			var checkFile func(string)
			checkFile = func(path string) {
				_, err := os.Stat(path)
				if os.IsNotExist(err) {
					createdFileObjs = append(createdFileObjs, path)
					checkFile(filepath.Dir(path) + "/")
				}
			}
			checkFile(path.Join(containerPath, dest))
			for i := len(createdFileObjs) - 1; i >= fileObsIndx; i-- {
				if strings.HasSuffix(createdFileObjs[i], "/") {
					os.Mkdir(createdFileObjs[i], os.ModePerm)
				} else {
					desc, _ := os.OpenFile(createdFileObjs[i], os.O_RDONLY|os.O_CREATE, 0666)
					defer desc.Close()
				}
			}
		}
		err = syscall.Mount(source, path.Join(containerPath, dest), "", syscall.MS_BIND, "")
		if err != nil {
			wwlog.Warn("BIND ERROR: %s", err)
		}
	}

	err = syscall.Chroot(containerPath)
	if err != nil {
		return errors.Wrap(err, "failed to chroot")
	}

	err = os.Chdir("/")
	if err != nil {
		return errors.Wrap(err, "failed to chdir")
	}

	err = syscall.Mount("/proc", "/proc", "proc", 0, "")
	if err != nil {
		return errors.Wrap(err, "failed to mount proc")
	}

	os.Setenv("PS1", fmt.Sprintf("[%s] Warewulf> ", containerName))
	os.Setenv("PATH", "/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin")
	os.Setenv("HISTFILE", "/dev/null")

	err = syscall.Exec(args[1], args[1:], os.Environ())
	if err != nil {
		wwlog.Error("%s", err)
		os.Exit(1)
	}
	fileStat, _ = os.Stat(path.Join(containerPath, "/etc/passwd"))
	unixStat = fileStat.Sys().(*syscall.Stat_t)
	if passwdTime.Before(time.Unix(int64(unixStat.Ctim.Sec), int64(unixStat.Ctim.Nsec))) {
		wwlog.Warn("/etc/passwd has been modified, maybe you want to run syncuser")
	}
	wwlog.Debug("passwd: %v", time.Unix(int64(unixStat.Ctim.Sec), int64(unixStat.Ctim.Nsec)))
	fileStat, _ = os.Stat(path.Join(containerPath, "/etc/group"))
	unixStat = fileStat.Sys().(*syscall.Stat_t)
	if groupTime.Before(time.Unix(int64(unixStat.Ctim.Sec), int64(unixStat.Ctim.Nsec))) {
		wwlog.Warn("/etc/group has been modified, maybe you want to run syncuser")
	}
	wwlog.Debug("group: %v", time.Unix(int64(unixStat.Ctim.Sec), int64(unixStat.Ctim.Nsec)))

	return nil
}
