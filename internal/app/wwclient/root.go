package wwclient

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/daemon"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/talos-systems/go-smbios/smbios"
	warewulfconf "github.com/warewulf/warewulf/internal/pkg/config"
	"github.com/warewulf/warewulf/internal/pkg/pidfile"
	"github.com/warewulf/warewulf/internal/pkg/util"
	"github.com/warewulf/warewulf/internal/pkg/wwlog"
)

var (
	rootCmd = &cobra.Command{
		Use:          "wwclient",
		Short:        "wwclient",
		Long:         "wwclient fetches the runtime overlay and puts it on the disk",
		RunE:         CobraRunE,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
	}
	DebugFlag       bool
	PIDFile         string
	Webclient       *http.Client
	WarewulfConfArg string
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&DebugFlag, "debug", "d", false, "Run with debugging messages enabled.")
	rootCmd.PersistentFlags().StringVarP(&PIDFile, "pidfile", "p", "/var/run/wwclient.pid", "PIDFile to use")
	rootCmd.PersistentFlags().StringVar(&WarewulfConfArg, "warewulfconf", "", "Set the warewulf configuration file")

}

// GetRootCommand returns the root cobra.Command for the application.
func GetRootCommand() *cobra.Command {
	// Run cobra
	return rootCmd
}

func CobraRunE(cmd *cobra.Command, args []string) (err error) {
	conf := warewulfconf.Get()
	if WarewulfConfArg != "" {
		err = conf.Read(WarewulfConfArg, false)
	} else if os.Getenv("WAREWULFCONF") != "" {
		err = conf.Read(os.Getenv("WAREWULFCONF"), false)
	} else {
		err = conf.Read(warewulfconf.ConfigFile, false)
	}
	if err != nil {
		return
	}
	pid, err := pidfile.Write(PIDFile)
	defer cleanUp()
	if err != nil && pid == -1 {
		wwlog.Warn("%v. starting new wwclient", err)
	} else if err != nil && pid > 0 {
		return errors.New("found pidfile " + PIDFile + " not starting")
	}

	if os.Args[0] == path.Join(conf.Paths.WWClientdir, "wwclient") {
		err := os.Chdir("/")
		if err != nil {
			return fmt.Errorf("failed to change dir: %w", err)
		}
		wwlog.Warn("updating live file system: cancel now if this is in error")
		time.Sleep(5000 * time.Millisecond)
	} else {
		fmt.Printf("Called via: %s\n", os.Args[0])
		fmt.Printf("Runtime overlay is being put in '/warewulf/wwclient-test' rather than '/'\n")
		fmt.Printf("For full functionality call with: %s\n", path.Join(conf.Paths.WWClientdir, "wwclient"))
		err := os.MkdirAll("/warewulf/wwclient-test", 0755)
		if err != nil {
			return fmt.Errorf("failed to create dir: %w", err)
		}

		err = os.Chdir("/warewulf/wwclient-test")
		if err != nil {
			return fmt.Errorf("failed to change dir: %w", err)
		}
	}

	localTCPAddr := net.TCPAddr{}
	if conf.WWClient != nil && conf.WWClient.Port > 0 {
		localTCPAddr.Port = int(conf.WWClient.Port)
		wwlog.Info("Running from configured port %d", conf.WWClient.Port)
	} else if conf.Warewulf.Secure() {
		// Setup local port to something privileged (<1024)
		localTCPAddr.Port = 987
		wwlog.Info("Running from trusted port: %d", localTCPAddr.Port)
	}

	Webclient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				LocalAddr: &localTCPAddr,
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       2 * time.Duration(conf.Warewulf.UpdateInterval) * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	var localUUID uuid.UUID
	var tag string
	smbiosDump, smbiosErr := smbios.New()
	if smbiosErr == nil {
		sysinfoDump := smbiosDump.SystemInformation()
		localUUID, _ = sysinfoDump.UUID()
		x := smbiosDump.SystemEnclosure()
		tag = strings.ReplaceAll(x.AssetTagNumber(), " ", "_")
		if tag == "Unknown" {
			dmiOut, err := exec.Command("dmidecode", "-s", "chassis-asset-tag").Output()
			if err == nil {
				chassisAssetTag := strings.TrimSpace(string(dmiOut))
				if chassisAssetTag != "" {
					tag = chassisAssetTag
				}
			}
		}
	} else {
		// Raspberry Pi serial and DUID locations
		// /sys/firmware/devicetree/base/serial-number
		// /sys/firmware/devicetree/base/chosen/rpi-duid
		piSerial, err := os.ReadFile("/sys/firmware/devicetree/base/serial-number")
		if err != nil {
			return fmt.Errorf("could not get SMBIOS info: %w", smbiosErr)
		}
		localUUID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("http://raspberrypi.com/serial-number/"+string(piSerial)))
		tag = "Unknown"
	}

	cmdline, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return fmt.Errorf("could not read from /proc/cmdline: %w", err)
	}

	wwid_tmp := strings.Split(string(cmdline), "wwid=")
	if len(wwid_tmp) < 2 {
		return fmt.Errorf("'wwid' is not defined in /proc/cmdline")
	}

	wwid := strings.Split(wwid_tmp[1], " ")[0]
	wwid = strings.TrimSuffix(wwid, "\n")

	// Dereference wwid from [interface] for cases that cannot have /proc/cmdline set by bootloader
	if string(wwid[0]) == "[" {
		iface := wwid[1 : len(wwid)-1]
		wwid_tmp, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/address", iface))
		if err != nil {
			return fmt.Errorf("'wwid' cannot be dereferenced from /sys/class/net: %w", err)
		}
		wwid = strings.TrimSuffix(string(wwid_tmp), "\n")
		wwlog.Info("Dereferencing wwid from [%s] to %s", iface, wwid)
	}

	duration := 300
	if conf.Warewulf.UpdateInterval > 0 {
		duration = conf.Warewulf.UpdateInterval
	}
	stopTimer := time.NewTimer(time.Duration(duration) * time.Second)
	// listen on SIGHUP
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for {
			sig := <-sigs
			switch sig {
			case syscall.SIGHUP:
				wwlog.Info("received signal: %s", sig)
				stopTimer.Stop()
				stopTimer.Reset(0)
			case syscall.SIGTERM, syscall.SIGINT:
				wwlog.Info("terminating wwclient, %v", sig)
				os.Exit(0)
			}
		}
	}()
	var finishedInitialSync bool = false
	ipaddr := os.Getenv("WW_IPADDR")
	if ipaddr == "" {
		ipaddr = conf.Ipaddr
	}
	var currentSum []byte
	for {
		currentSum = updateSystem(ipaddr, conf.Warewulf.Port, wwid, tag, localUUID, currentSum)
		if !finishedInitialSync {
			// ignore error and status here, as this wouldn't change anything
			_, _ = daemon.SdNotify(false, daemon.SdNotifyReady)
			finishedInitialSync = true
		}

		<-stopTimer.C
		stopTimer.Reset(time.Duration(duration) * time.Second)
	}
}

// check if currentSum matches the sum returned from the server, if both sum are same, do nothingm if
// sum differs update the overlay. Return error and empty sum on any error.
func updateSystem(ipaddr string, port int, wwid string, tag string, localUUID uuid.UUID, currentSum []byte) []byte {
	var resp *http.Response
	counter := 0

	if len(currentSum) > 0 {
		var err error
		values := &url.Values{}
		values.Set("assetkey", tag)
		values.Set("uuid", localUUID.String())
		values.Set("stage", "runtime_check")
		values.Set("checksum", string(currentSum))

		getURL := &url.URL{
			Scheme:   "http",
			Host:     fmt.Sprintf("%s:%d", ipaddr, port),
			Path:     fmt.Sprintf("provision/%s", wwid),
			RawQuery: values.Encode(),
		}
		wwlog.Debug("making checksum request: %s", getURL)
		for {
			resp, err = Webclient.Get(getURL.String())
			if err == nil {
				break
			} else {
				if counter > 60 {
					counter = 0
				}
				if counter == 0 {
					wwlog.Error("%s", err)
				}
				counter++
			}
			time.Sleep(1000 * time.Millisecond)
		}

		if resp.StatusCode == http.StatusOK {
			remoteSum, err := io.ReadAll(resp.Body)
			if err != nil {
				wwlog.Error("could not read checksum from response: %s", err)
				resp.Body.Close()
				return []byte{}
			}
			resp.Body.Close()
			if bytes.Equal(bytes.TrimSpace(remoteSum), currentSum) {
				wwlog.Info("runtime overlay is current")
				return currentSum
			}
		} // Checksum is different or not found, so fall through to update
	}

	counter = 0
	for {
		var err error
		values := &url.Values{}
		values.Set("assetkey", tag)
		values.Set("uuid", localUUID.String())
		values.Set("stage", "runtime")
		values.Set("compress", "gz")
		getURL := &url.URL{
			Scheme:   "http",
			Host:     fmt.Sprintf("%s:%d", ipaddr, port),
			Path:     fmt.Sprintf("provision/%s", wwid),
			RawQuery: values.Encode(),
		}
		wwlog.Debug("making request: %s", getURL)
		resp, err = Webclient.Get(getURL.String())
		if err == nil {
			break
		} else {
			if counter > 60 {
				counter = 0
			}
			if counter == 0 {
				wwlog.Error("%s", err)
			}
			counter++
		}
		time.Sleep(1000 * time.Millisecond)
	}

	if resp.StatusCode == http.StatusNoContent {
		wwlog.Info("no runtime overlay available for this node")
		return []byte{}
	}

	if resp.StatusCode != http.StatusOK {
		wwlog.Warn("not applying runtime overlay: got status code: %d", resp.StatusCode)
		time.Sleep(60000 * time.Millisecond)
		return []byte{}
	}

	tmpFile, err := os.CreateTemp(os.TempDir(), "ww-runtime-")
	if err != nil {
		wwlog.Error("could not create temporary file: %s", err)
		return []byte{}
	}
	defer os.Remove(tmpFile.Name())

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		wwlog.Error("failed to write runtime overlay to temporary file: %s", err)
		tmpFile.Close()
		return []byte{}
	}
	tmpFile.Close()
	resp.Body.Close()

	r, err := os.Open(tmpFile.Name())
	if err != nil {
		wwlog.Error("could not open temporary file for reading: %s", err)
		return []byte{}
	}
	defer r.Close()

	newSum, err := util.HashFile(r)
	if err != nil {
		wwlog.Error("could not hash runtime overlay: %s", err)
		return []byte{}
	}

	wwlog.Info("applying runtime overlay")
	_, err = r.Seek(0, 0)
	if err != nil {
		wwlog.Error("could not seek in temporary file: %s", err)
		return []byte{}
	}

	command := exec.Command("/bin/sh", "-c", "gzip -dc | cpio -iu")
	command.Stdin = r
	err = command.Run()
	if err != nil {
		wwlog.Error("failed running cpio: %s", err)
		return []byte{}
	}
	return []byte(newSum)
}

func cleanUp() {
	err := pidfile.Remove(PIDFile)
	if err != nil {
		wwlog.Error("could not remove pidfile: %s", err)
	}
}
