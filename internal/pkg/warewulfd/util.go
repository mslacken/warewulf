package warewulfd

import (
	"bufio"
	"net/http"
	"os"
	"strings"

	nodepkg "github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/hpcng/warewulf/internal/pkg/overlay"
	"github.com/hpcng/warewulf/internal/pkg/util"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
)

func sendFile(
	w http.ResponseWriter,
	req *http.Request,
	filename string,
	sendto string) error {

	fd, err := os.Open(filename)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	defer fd.Close()

	stat, err := fd.Stat()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	http.ServeContent(
		w,
		req,
		filename,
		stat.ModTime(),
		fd)

	wwlog.Send("%15s: %s", sendto, filename)

	return nil
}

func getOverlayFile(
	nodeId string,
	context string,
	stage_overlays []string,
	autobuild bool) (stage_file string, err error) {

	stage_file = overlay.OverlayImage(nodeId, context, stage_overlays)
	err = nil

	build := !util.IsFile(stage_file)

	if !build && autobuild {
		build = util.PathIsNewer(stage_file, nodepkg.ConfigFile)

		for _, overlayname := range stage_overlays {
			build = build || util.PathIsNewer(stage_file, overlay.OverlaySourceDir(overlayname))
		}
	}

	if build {
		wwlog.Serv("BUILD %15s, overlays %v", nodeId, stage_overlays)

		args := []string{"overlay", "build"}

		for _, overlayname := range stage_overlays {
			args = append(args, "-O", overlayname)
		}

		args = append(args, nodeId)

		out, err := util.RunWWCTL(args...)

		if err != nil {
			wwlog.Error("Failed to build overlay: %s, %s, %s\n%s",
				nodeId, stage_overlays, stage_file, string(out))
		}
	}

	return
}

/*
returns the mac address if it has an entry in the arp cache
*/

func ArpFind(ip string) (mac string) {
	arpCache, err := os.Open("/proc/net/arp")
	if err != nil {
		return
	}
	defer arpCache.Close()

	scanner := bufio.NewScanner(arpCache)
	scanner.Scan()
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if strings.EqualFold(fields[0], ip) {
			return fields[3]
		}
	}
	return
}
