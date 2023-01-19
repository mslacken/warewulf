module github.com/hpcng/warewulf

replace libguestfs.org/guestfs => ./external/libguestfs.org/guestfs

go 1.16

require (
	github.com/brotherpowers/ipsubnet v0.0.0-20170914094241-30bc98f0a5b1
	github.com/containers/image/v5 v5.7.0
	github.com/containers/storage v1.30.0
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/creasty/defaults v1.5.2
	github.com/fatih/color v1.13.0
	github.com/golang/glog v1.0.0
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.10.0
	github.com/manifoldco/promptui v0.8.0
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/opencontainers/umoci v0.4.6
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.7.0
	github.com/talos-systems/go-smbios v0.1.1
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	google.golang.org/genproto v0.0.0-20220317150908-0efb43f6373e
	google.golang.org/grpc v1.45.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.2.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/yaml.v2 v2.4.0
	libguestfs.org/guestfs v1.0.0
)
