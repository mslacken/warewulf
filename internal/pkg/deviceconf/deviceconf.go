package deviceconf

type DiskConfig struct {
	Partitions struct {
		Config struct {
			Label string `yaml:"label"`
		} `yaml:"config"`
		Devices map[string]Devices `yaml:"devices"`
	} `yaml:"partitions"`
	Filesystem map[string]FilesystemDesc `yaml:"filesystems,omitempty"`
}

type Devices struct {
	Partitions []Partition `yaml:"partitions"`
}

type Partition struct {
	Number string `yaml:"number"`
	Size   string `yaml:"size"`
	Type   string `yaml:"type"`
}

type FilesystemDesc struct {
	FilesystemType string `yaml:"filesystem"`
	MountPoint     string `yaml:"mountpoint"`
	Format         bool   `yaml:"format"`
	MountOptions   string `yaml:"mount_options"`
}

func ParseDeviceConfig(configName string) error {
	return nil
}
