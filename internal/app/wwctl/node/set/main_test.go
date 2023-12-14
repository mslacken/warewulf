package set

import (
	"bytes"
	"testing"

	"github.com/hpcng/warewulf/internal/pkg/testenv"
	"github.com/hpcng/warewulf/internal/pkg/warewulfd"
	"github.com/stretchr/testify/assert"
)

type test_description struct {
	name    string
	args    []string
	wantErr bool
	stdout  string
	inDB    string
	outDb   string
}

func run_test(t *testing.T, test test_description) {
	env := testenv.New(t)
	defer env.RemoveAll(t)
	//wwlog.SetLogLevel(wwlog.DEBUG)
	env.WriteFile(t, "etc/warewulf/nodes.conf", test.inDB)
	warewulfd.SetNoDaemon()
	name := test.name
	if name == "" {
		name = t.Name()
	}
	t.Run(name, func(t *testing.T) {
		baseCmd := GetCommand()
		test.args = append(test.args, "--yes")
		baseCmd.SetArgs(test.args)
		buf := new(bytes.Buffer)
		baseCmd.SetOut(buf)
		baseCmd.SetErr(buf)
		err := baseCmd.Execute()
		if test.wantErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, buf.String(), test.stdout)
			content := env.ReadFile(t, "etc/warewulf/nodes.conf")
			assert.Equal(t, test.outDb, content)
		}
	})
}

func Test_Single_Node_Change_Profile(t *testing.T) {
	test := test_description{
		args:    []string{"--profile=foo", "n01"},
		wantErr: false,
		stdout:  "",
		inDB: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default`,
		outDb: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - foo
`}
	run_test(t, test)
}

func Test_Multiple_Set_Tests(t *testing.T) {
	tests := []test_description{
		{name: "single node change profile",
			args:    []string{"--profile=foo", "n01"},
			wantErr: false,
			stdout:  "",
			inDB: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default`,
			outDb: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - foo
`},
		{name: "multiple nodes change profile",
			args:    []string{"--profile=foo", "n0[1-2]"},
			wantErr: false,
			stdout:  "",
			inDB: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
  n02:
    profiles:
    - default`,
			outDb: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - foo
  n02:
    profiles:
    - foo
`},
		{name: "single node set ipmitag",
			args:    []string{"--ipmitagadd", "foo=baar", "n01"},
			wantErr: false,
			stdout:  "",
			inDB: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default`,
			outDb: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    ipmi:
      tags:
        foo: baar
    profiles:
    - default
`},
		{name: "single node delete tag",
			args:    []string{"--tagdel", "tag1", "n01"},
			wantErr: false,
			stdout:  "",
			inDB: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    tags:
      tag1: value1
      tag2: value2`,
			outDb: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    tags:
      tag2: value2
`},
		{name: "single node set fs,part and disk",
			args:    []string{"--fsname=var", "--fspath=/var", "--fsformat=btrfs", "--partname=var", "--diskname=/dev/vda", "n01"},
			wantErr: false,
			stdout:  "",
			inDB: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
`,
			outDb: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    disks:
      /dev/vda:
        partitions:
          var: {}
    filesystems:
      /dev/disk/by-partlabel/var:
        format: btrfs
        path: /var
`},
		{name: "single delete not existing fs",
			args:    []string{"--fsdel=foo", "n01"},
			wantErr: true,
			stdout:  "",
			inDB: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    disks:
      /dev/vda:
        partitions:
          var: {}
        path: /var
    filesystems:
      /dev/disk/by-partlabel/var:
        format: btrfs
        path: /var
`,
			outDb: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    disks:
      /dev/vda:
        partitions:
          var: {}
    filesystems:
      /dev/disk/by-partlabel/var:
        format: btrfs
        path: /var
`},
		{name: "single node delete existing fs",
			args:    []string{"--fsdel=/dev/disk/by-partlabel/var", "n01"},
			wantErr: false,
			stdout:  "",
			inDB: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    disks:
      /dev/vda:
        partitions:
          var: {}
        path: /var
    filesystems:
      /dev/disk/by-partlabel/var:
        format: btrfs
        path: /var
`,
			outDb: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    disks:
      /dev/vda:
        partitions:
          var: {}
`},
		{name: "single node delete existing partition",
			args:    []string{"--partdel=var", "n01"},
			wantErr: false,
			stdout:  "",
			inDB: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    disks:
      /dev/vda:
        partitions:
          var: {}
        path: /var
    filesystems:
      /dev/disk/by-partlabel/var:
        format: btrfs
        path: /var
`,
			outDb: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    disks:
      /dev/vda: {}
    filesystems:
      /dev/disk/by-partlabel/var:
        format: btrfs
        path: /var
`},
		{name: "single node delete existing disk",
			args:    []string{"--diskdel=/dev/vda", "n01"},
			wantErr: false,
			stdout:  "",
			inDB: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    disks:
      /dev/vda:
        partitions:
          var: {}
        path: /var
    filesystems:
      /dev/disk/by-partlabel/var:
        format: btrfs
        path: /var
`,
			outDb: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    filesystems:
      /dev/disk/by-partlabel/var:
        format: btrfs
        path: /var
`},
		{name: "single node set mtu",
			args:    []string{"--mtu", "1234", "--netname=mynet", "n01"},
			wantErr: false,
			stdout:  "",
			inDB: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default`,
			outDb: `WW_INTERNAL: 43
nodeprofiles:
  default:
    comment: testit
nodes:
  n01:
    profiles:
    - default
    network devices:
      mynet:
        mtu: "1234"
`},
		{name: "single node set ipmitag",
			args:    []string{"--tagadd", "nodetag1=nodevalue1", "n01"},
			wantErr: false,
			stdout:  "",
			inDB: `WW_INTERNAL: 43
nodeprofiles:
  p1:
    comment: testit 1
    tags:
      p1tag1: p1val1
  p2:
    comment: testit 1
    tags:
      p2tag2: p1val2
nodes:
  n01:
    profiles:
    - p1
    - p2`,
			outDb: `WW_INTERNAL: 43
nodeprofiles:
  p1:
    comment: testit 1
    tags:
      p1tag1: p1val1
  p2:
    comment: testit 1
    tags:
      p2tag2: p1val2
nodes:
  n01:
    profiles:
    - p1
    - p2
    tags:
      nodetag1: nodevalue1
`},
	}
	for _, tt := range tests {
		run_test(t, tt)
	}
}
