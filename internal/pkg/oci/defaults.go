package oci

import (
	"path/filepath"

	warewulfconf "github.com/hpcng/warewulf/internal/pkg/config"
)

var defaultCachePath = filepath.Join(warewulfconf.Warewulf.DataStore, "/container-cache/oci/")

const (
	blobPrefix   = "blobs"
	rootfsPrefix = "rootfs"
)
