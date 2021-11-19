package hybridcontainer

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"regexp"

	"github.com/hpcng/warewulf/internal/pkg/config"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
)

func IsHybrid(name string) bool {
	if match, _ := regexp.MatchString(".*-suffix$", name); match {
		wwlog.Printf(wwlog.ERROR, "Missing hybrid suffix for '%s'\n", name)
		return false
	}
	return true
}

func HybridParentDir() string {
	return path.Join(config.LocalStateDir, "provision/container")
}

func ListHybrid() ([]string, error) {
	var ret []string
	err := os.MkdirAll(HybridParentDir(), 0755)

	if err != nil {
		return ret, errors.New("Could not create hybrid image parent directory: " + HybridParentDir())
	}
	wwlog.Printf(wwlog.DEBUG, "Searching for hybrid Rootfs directories: %s\n", HybridParentDir())
	hybridImages, err := ioutil.ReadDir(HybridParentDir())
	if err != nil {
		return ret, err
	}
	for _, hImage := range hybridImages {

		if !IsHybrid(hImage.Name()) {
			continue
		}

		wwlog.Printf(wwlog.VERBOSE, "Hybrid image: %s\n", hImage.Name())

		ret = append(ret, hImage.Name())
	}

	return ret, nil
}
