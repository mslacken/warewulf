package warewulfd

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/google/go-attestation/attest"
	warewulfconf "github.com/warewulf/warewulf/internal/pkg/config"
	"github.com/warewulf/warewulf/internal/pkg/image"
	"github.com/warewulf/warewulf/internal/pkg/kernel"
	nodedb "github.com/warewulf/warewulf/internal/pkg/node"
	"github.com/warewulf/warewulf/internal/pkg/overlay"
	"github.com/warewulf/warewulf/internal/pkg/tpm"
	"github.com/warewulf/warewulf/internal/pkg/util"
	"github.com/warewulf/warewulf/internal/pkg/wwlog"
)

type templateVars struct {
	Message       string
	WaitTime      string
	Hostname      string
	Fqdn          string
	Id            string
	Cluster       string
	ImageName     string
	Ipxe          string
	Hwaddr        string
	Ipaddr        string
	Ipaddr6       string
	Port          string
	Authority     string
	KernelArgs    string
	KernelVersion string
	Root          string
	Https         bool
	Tags          map[string]string
	NetDevs       map[string]*nodedb.NetDev
}

func ProvisionSend(w http.ResponseWriter, req *http.Request) {
	wwlog.Debug("Requested URL: %s", req.URL.String())
	conf := warewulfconf.Get()
	rinfo, err := parseReq(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		wwlog.ErrorExc(err, "Bad status")
		return
	}

	wwlog.Debug("stage: %s", rinfo.stage)

	wwlog.Info("request from hwaddr:%s ipaddr:%s | stage:%s", rinfo.hwaddr, req.RemoteAddr, rinfo.stage)

	if (rinfo.stage == "runtime" || len(rinfo.overlay) > 0) && conf.Warewulf.Secure() {
		if rinfo.remoteport >= 1024 {
			wwlog.Denied("Non-privileged port: %s", req.RemoteAddr)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	status_stages := map[string]string{
		"efiboot":   "EFI",
		"ipxe":      "IPXE",
		"kernel":    "KERNEL",
		"system":    "SYSTEM_OVERLAY",
		"runtime":   "RUNTIME_OVERLAY",
		"initramfs": "INITRAMFS"}

	status_stage := status_stages[rinfo.stage]
	var stage_file string

	// TODO: when module version is upgraded to go1.18, should be 'any' type
	var tmpl_data *templateVars

	remoteNode, err := GetNodeOrSetDiscoverable(rinfo.hwaddr, conf.Warewulf.AutobuildOverlays())
	if err != nil && err != nodedb.ErrNoUnconfigured {
		wwlog.ErrorExc(err, "")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	if remoteNode.AssetKey != "" && remoteNode.AssetKey != rinfo.assetkey {
		w.WriteHeader(http.StatusUnauthorized)
		wwlog.Denied("incorrect asset key: node %s: %s", remoteNode.Id(), rinfo.assetkey)
		updateStatus(remoteNode.Id(), status_stage, "BAD_ASSET", rinfo.ipaddr)
		return
	}

	if !remoteNode.Valid() {
		wwlog.Error("%s (unknown/unconfigured node)", rinfo.hwaddr)
		if rinfo.stage == "ipxe" {
			stage_file = path.Join(conf.Paths.Sysconfdir, "/warewulf/ipxe/unconfigured.ipxe")
			tmpl_data = &templateVars{
				Hwaddr: rinfo.hwaddr}
		}

	} else if rinfo.stage == "ipxe" {
		template := remoteNode.Ipxe
		if template == "" {
			template = "default"
		}
		stage_file = path.Join(conf.Paths.Sysconfdir, "warewulf/ipxe", template+".ipxe")
		kernelArgs := ""
		kernelVersion := ""
		if remoteNode.Kernel != nil {
			kernelArgs = strings.Join(remoteNode.Kernel.Args, " ")
			kernelVersion = remoteNode.Kernel.Version
		}
		if kernelVersion == "" {
			if kernel_ := kernel.FromNode(&remoteNode); kernel_ != nil {
				kernelVersion = kernel_.Version()
			}
		}
		authority := fmt.Sprintf("%s:%d", conf.Ipaddr, conf.Warewulf.Port)
		ipaddr6 := ""
		if confIpaddr6, err := netip.ParseAddr(conf.Ipaddr6); err == nil {
			ipaddr6 = confIpaddr6.String()
		}
		if rinfoIpaddr, err := netip.ParseAddr(rinfo.ipaddr); err == nil {
			if rinfoIpaddr.Is6() {
				if ipaddr6 != "" {
					authority = fmt.Sprintf("[%s]:%d", ipaddr6, conf.Warewulf.Port)
				} else {
					wwlog.Error("No valid IPv6 address configured, but request is IPv6")
				}
			}
		} else {
			wwlog.Error("Could not parse request IP address: %s", rinfo.ipaddr)
		}
		tmpl_data = &templateVars{
			Id:            remoteNode.Id(),
			Cluster:       remoteNode.ClusterName,
			Fqdn:          remoteNode.Id(),
			Ipaddr:        conf.Ipaddr,
			Ipaddr6:       ipaddr6,
			Port:          strconv.Itoa(conf.Warewulf.Port),
			Authority:     authority,
			Hostname:      remoteNode.Id(),
			Hwaddr:        rinfo.hwaddr,
			ImageName:     remoteNode.ImageName,
			KernelArgs:    kernelArgs,
			KernelVersion: kernelVersion,
			Root:          remoteNode.Root,
			NetDevs:       remoteNode.NetDevs,
			Tags:          remoteNode.Tags}
	} else if rinfo.stage == "kernel" {
		kernel_ := kernel.FromNode(&remoteNode)
		if kernel_ == nil {
			wwlog.Error("No kernel found for node %s", remoteNode.Id())
		} else {
			stage_file = kernel_.FullPath()
			if stage_file == "" {
				wwlog.Error("No kernel path found for node %s", remoteNode.Id())
			}
		}

	} else if rinfo.stage == "image" {
		if remoteNode.ImageName != "" {
			stage_file = image.ImageFile(remoteNode.ImageName)
		} else {
			wwlog.Warn("No image set for node %s", remoteNode.Id())
		}

	} else if rinfo.stage == "system" || rinfo.stage == "runtime" {
		var context string
		var request_overlays []string

		if len(rinfo.overlay) > 0 {
			request_overlays = strings.Split(rinfo.overlay, ",")
		} else {
			context = rinfo.stage
		}
		stage_file, err = getOverlayFile(
			remoteNode,
			context,
			request_overlays,
			conf.Warewulf.AutobuildOverlays())

		if err != nil {
			if errors.Is(err, overlay.ErrDoesNotExist) {
				w.WriteHeader(http.StatusNotFound)
				wwlog.ErrorExc(err, "")
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			wwlog.ErrorExc(err, "")
			return
		}
	} else if rinfo.stage == "efiboot" {
		wwlog.Debug("requested method: %s", req.Method)
		imageName := remoteNode.ImageName
		switch rinfo.efifile {
		case "shim.efi":
			stage_file = image.ShimFind(imageName)
			if stage_file == "" {
				wwlog.Error("couldn't find shim.efi for %s", imageName)
				w.WriteHeader(http.StatusNotFound)
				return
			}
		case "grub.efi", "grub-tpm.efi", "grubx64.efi", "grubia32.efi", "grubaa64.efi", "grubarm.efi":
			stage_file = image.GrubFind(imageName)
			if stage_file == "" {
				wwlog.Error("could't find grub*.efi for %s", imageName)
				w.WriteHeader(http.StatusNotFound)
				return
			}
		case "grub.cfg":
			stage_file = path.Join(conf.Paths.Sysconfdir, "warewulf/grub/grub.cfg.ww")
			kernelArgs := ""
			kernelVersion := ""
			if remoteNode.Kernel != nil {
				kernelArgs = strings.Join(remoteNode.Kernel.Args, " ")
				kernelVersion = remoteNode.Kernel.Version
			}
			if kernelVersion == "" {
				if kernel_ := kernel.FromNode(&remoteNode); kernel_ != nil {
					kernelVersion = kernel_.Version()
				}
			}
			authority := fmt.Sprintf("%s:%d", conf.Ipaddr, conf.Warewulf.Port)
			ipaddr6 := ""
			if confIpaddr6, err := netip.ParseAddr(conf.Ipaddr6); err == nil {
				ipaddr6 = confIpaddr6.String()
			}
			if rinfoIpaddr, err := netip.ParseAddr(rinfo.ipaddr); err == nil {
				if rinfoIpaddr.Is6() {
					if ipaddr6 != "" {
						authority = fmt.Sprintf("[%s]:%d", ipaddr6, conf.Warewulf.Port)
					} else {
						wwlog.Error("No valid IPv6 address configured, but request is IPv6")
					}
				}
			} else {
				wwlog.Error("Could not parse request IP address: %s", rinfo.ipaddr)
			}
			tmpl_data = &templateVars{
				Id:            remoteNode.Id(),
				Cluster:       remoteNode.ClusterName,
				Fqdn:          remoteNode.Id(),
				Ipaddr:        conf.Ipaddr,
				Ipaddr6:       ipaddr6,
				Port:          strconv.Itoa(conf.Warewulf.Port),
				Https:         conf.Warewulf.EnableTLS(),
				Authority:     authority,
				Hostname:      remoteNode.Id(),
				Hwaddr:        rinfo.hwaddr,
				ImageName:     remoteNode.ImageName,
				Ipxe:          remoteNode.Ipxe,
				KernelArgs:    kernelArgs,
				KernelVersion: kernelVersion,
				Root:          remoteNode.Root,
				NetDevs:       remoteNode.NetDevs,
				Tags:          remoteNode.Tags}
			if stage_file == "" {
				wwlog.Error("could't find grub.cfg template for %s", imageName)
				w.WriteHeader(http.StatusNotFound)
				return
			}
		default:
			wwlog.ErrorExc(fmt.Errorf("could't find efiboot file: %s", rinfo.efifile), "")
		}
	} else if rinfo.stage == "shim" {
		if remoteNode.ImageName != "" {
			stage_file = image.ShimFind(remoteNode.ImageName)

			if stage_file == "" {
				wwlog.Error("No kernel found for image %s", remoteNode.ImageName)
			}
		} else {
			wwlog.Warn("No image set for this %s", remoteNode.Id())
		}
	} else if rinfo.stage == "grub" {
		if remoteNode.ImageName != "" {
			stage_file = image.GrubFind(remoteNode.ImageName)
			if stage_file == "" {
				wwlog.Error("No grub found for image %s", remoteNode.ImageName)
			}
		} else {
			wwlog.Warn("No conainer set for node %s", remoteNode.Id())
		}
	} else if rinfo.stage == "initramfs" {
		if kernel_ := kernel.FromNode(&remoteNode); kernel_ != nil {
			if kver := kernel_.Version(); kver != "" {
				if initramfs := image.FindInitramfs(remoteNode.ImageName, kver); initramfs != nil {
					stage_file = initramfs.FullPath()
				} else {
					wwlog.Error("No initramfs found for kernel %s in image %s", kver, remoteNode.ImageName)
				}
			} else {
				wwlog.Error("No initramfs found: unable to determine kernel version for node %s", remoteNode.Id())
			}
		} else {
			wwlog.Error("No initramfs found: unable to find kernel for node %s", remoteNode.Id())
		}
	}

	wwlog.Serv("stage_file '%s'", stage_file)

	if util.IsFile(stage_file) {
		var contentBytes []byte

		if tmpl_data != nil {
			if rinfo.compress != "" {
				wwlog.Error("Unsupported %s compressed version for file: %s",
					rinfo.compress, stage_file)
				w.WriteHeader(http.StatusNotFound)
				return
			}

			// Create a template with the Sprig functions.
			tmpl := template.New(filepath.Base(stage_file)).Funcs(sprig.TxtFuncMap())

			// Parse the template.
			parsedTmpl, err := tmpl.ParseFiles(stage_file)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				wwlog.ErrorExc(err, "")
				return
			}

			// template engine writes file to buffer in case rendering fails
			var buf bytes.Buffer

			err = parsedTmpl.Execute(&buf, tmpl_data)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				wwlog.ErrorExc(err, "")
				return
			}

			w.Header().Set("Content-Type", "text")
			w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
			contentBytes = buf.Bytes()
			_, err = buf.WriteTo(w)
			if err != nil {
				wwlog.ErrorExc(err, "")
			}

			wwlog.Info("send %s -> %s", stage_file, remoteNode.Id())

		} else {
			if rinfo.compress == "gz" {
				stage_file += ".gz"

				if !util.IsFile(stage_file) {
					wwlog.Error("unprepared for compressed version of file %s",
						stage_file)
					w.WriteHeader(http.StatusNotFound)
					return
				}
			} else if rinfo.compress != "" {
				wwlog.Error("unsupported %s compressed version of file %s",
					rinfo.compress, stage_file)
				w.WriteHeader(http.StatusNotFound)
			}

			// Read file content for checksum
			fileBytes, err := os.ReadFile(stage_file)
			if err != nil {
				wwlog.ErrorExc(err, "")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			contentBytes = fileBytes

			err = sendFile(w, req, stage_file, remoteNode.Id())
			if err != nil {
				wwlog.ErrorExc(err, "")
				return
			}
		}

		// Calculate checksum and update TPM log
		sum := sha256.Sum256(contentBytes)
		checksum := fmt.Sprintf("%x", sum)

		err = updateTPMLogs(remoteNode.Id(), stage_file, stage_file, checksum)
		if err != nil {
			wwlog.Error("Failed to update TPM logs: %v", err)
		}

		updateStatus(remoteNode.Id(), status_stage, path.Base(stage_file), rinfo.ipaddr)

	} else if stage_file == "" {
		w.WriteHeader(http.StatusBadRequest)
		wwlog.Error("No resource selected")
		updateStatus(remoteNode.Id(), status_stage, "BAD_REQUEST", rinfo.ipaddr)

	} else {
		w.WriteHeader(http.StatusNotFound)
		wwlog.Error("Not found: %s", stage_file)
		updateStatus(remoteNode.Id(), status_stage, "NOT_FOUND", rinfo.ipaddr)
	}

}

func TPMReceive(w http.ResponseWriter, req *http.Request) {
	wwlog.Debug("Requested URL: %s", req.URL.String())

	wwidRecv := req.URL.Query().Get("wwid")
	if wwidRecv == "" {
		wwlog.Error("TPM receive: wwid parameter missing")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validate node exists
	nodes, err := nodedb.New()
	if err != nil {
		wwlog.Error("Failed to load node configuration: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Check if the node exists by ID, IP or HW address
	node, err := nodes.GetNodeOnly(wwidRecv)
	if err != nil {
		if node, err = nodes.FindByIpaddr(wwidRecv); err != nil {
			if node, err = nodes.FindByHwaddr(wwidRecv); err != nil {
				wwlog.Error("Node not found: %s", wwidRecv)
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		wwlog.Error("Failed to read request body: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var newQuote tpm.Quote
	err = json.Unmarshal(body, &newQuote)
	if err != nil {
		wwlog.Error("Failed to unmarshal JSON quote: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	newQuote.ID = wwidRecv
	newQuote.Modified = time.Now()

	conf := warewulfconf.Get()
	tpmDir := filepath.Join(conf.Paths.OverlayProvisiondir(), node.GetId())
	tpmPath := filepath.Join(tpmDir, "tpm.json")

	if util.IsFile(tpmPath) {
		data, err := os.ReadFile(tpmPath)
		if err == nil {
			var existingQuote tpm.Quote
			if err := json.Unmarshal(data, &existingQuote); err == nil {
				newQuote.Logs = existingQuote.Logs
			}
		}
	}

	out, err := json.MarshalIndent(newQuote, "", "  ")
	if err != nil {
		wwlog.Error("Failed to marshal TPM quote: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = os.WriteFile(tpmPath, out, 0644)
	if err != nil {
		wwlog.Error("Failed to write TPM quote: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	wwlog.Info("Stored TPM quote for node %s", newQuote.ID)
	w.WriteHeader(http.StatusOK)
}

func TPMChallengeSend(w http.ResponseWriter, req *http.Request) {
	wwlog.Debug("Requested URL: %s", req.URL.String())

	wwidRecv := req.URL.Query().Get("wwid")
	if wwidRecv == "" {
		wwlog.Error("TPM challenge send: wwid parameter missing")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	nodes, err := nodedb.New()
	if err != nil {
		wwlog.Error("Failed to load node configuration: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	node, err := nodes.GetNodeOnly(wwidRecv)
	if err != nil {
		if node, err = nodes.FindByIpaddr(wwidRecv); err != nil {
			if node, err = nodes.FindByHwaddr(wwidRecv); err != nil {
				wwlog.Error("Node not found: %s", wwidRecv)
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}
	}

	conf := warewulfconf.Get()
	tpmPath := filepath.Join(conf.Paths.OverlayProvisiondir(), node.GetId(), "tpm.json")

	if !util.IsFile(tpmPath) {
		wwlog.Error("No TPM quote found for node %s", node.GetId())
		w.WriteHeader(http.StatusNotFound)
		return
	}

	data, err := os.ReadFile(tpmPath)
	if err != nil {
		wwlog.Error("Failed to read TPM quote: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var existingQuote tpm.Quote
	err = json.Unmarshal(data, &existingQuote)
	if err != nil {
		wwlog.Error("Failed to unmarshal TPM quote: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ekPubBytes, err := base64.StdEncoding.DecodeString(existingQuote.EKPub)
	if err != nil {
		wwlog.Error("Failed to decode EKPub for node %s: %s", node.GetId(), err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	akPubBytes, err := base64.StdEncoding.DecodeString(existingQuote.AKPub)
	if err != nil {
		wwlog.Error("Failed to decode AKPub for node %s: %s", node.GetId(), err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	akPub, err := x509.ParsePKIXPublicKey(akPubBytes)
	if err != nil {
		wwlog.Error("Failed to parse AK public key for node %s: %s", node.GetId(), err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ekPub, err := x509.ParsePKIXPublicKey(ekPubBytes)
	if err != nil {
		wwlog.Error("Failed to parse EK public key for node %s: %s", node.GetId(), err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	akPubDER, err := x509.MarshalPKIXPublicKey(akPub)
	if err != nil {
		wwlog.Error("Failed to marshal AK public key for node %s: %s", node.GetId(), err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	akAttestParams := attest.AttestationParameters{
		Public: akPubDER,
		// Other fields like Name, AttestedCreationInfo would ideally come from the client during AK creation
	}

	activationParams := attest.ActivationParameters{
		EK: ekPub,
		AK: akAttestParams,
	}

	secret, encryptedCredential, err := activationParams.Generate()
	if err != nil {
		wwlog.Error("Error generating Credential Activation Challenge for node %s: %s", node.GetId(), err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	newChallenge := tpm.Challenge{
		EncryptedCredential: *encryptedCredential,
		Secret:              secret,
		ID:                  node.GetId(),
	}

	out, err := json.MarshalIndent(newChallenge, "", "  ")
	if err != nil {
		wwlog.Error("Failed to marshal TPM challenge: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	challengePath := filepath.Join(conf.Paths.OverlayProvisiondir(), node.GetId(), "tpm_challenge.json")
	err = os.WriteFile(challengePath, out, 0644)
	if err != nil {
		wwlog.Error("Failed to write TPM challenge: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newChallenge.EncryptedCredential)

	wwlog.Info("Sent TPM challenge for node %s", node.GetId())
}

func updateTPMLogs(nodeId, filename, source, checksum string) error {
	conf := warewulfconf.Get()
	tpmPath := filepath.Join(conf.Paths.OverlayProvisiondir(), nodeId, "tpm.json")

	if !util.IsFile(tpmPath) {
		return nil
	}

	data, err := os.ReadFile(tpmPath)
	if err != nil {
		return err
	}

	var quote tpm.Quote
	err = json.Unmarshal(data, &quote)
	if err != nil {
		return err
	}

	// Check if log entry already exists
	found := false
	for i, log := range quote.Logs {
		if log.Filename == filename {
			quote.Logs[i].Checksum = checksum
			quote.Logs[i].Source = source
			found = true
			break
		}
	}

	if !found {
		quote.Logs = append(quote.Logs, tpm.FileLog{
			Filename: filename,
			Source:   source,
			Checksum: checksum,
		})
	}

	out, err := json.MarshalIndent(quote, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(tpmPath, out, 0644)
}
