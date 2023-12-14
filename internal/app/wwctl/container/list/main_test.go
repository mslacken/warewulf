package list

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/hpcng/warewulf/internal/pkg/api/routes/wwapiv1"
	"github.com/hpcng/warewulf/internal/pkg/testenv"
	"github.com/hpcng/warewulf/internal/pkg/warewulfd"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/stretchr/testify/assert"
)

func Test_List(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		stdout   string
		inDb     string
		mockFunc func()
	}{
		{
			name: "container list test",
			args: []string{},
			stdout: `  CONTAINER NAME  NODES  KERNEL VERSION  CREATION TIME        MODIFICATION TIME    SIZE
 test            1      kernel          01 Jan 70 01:00 CET  01 Jan 70 01:00 CET  1 B`,
			inDb: `WW_INTERNAL: 43
nodeprofiles:
  default: {}
nodes:
  n01:
    profiles:
    - default
`,
			mockFunc: func() {
				containerList = func() (containerInfo []*wwapiv1.ContainerInfo, err error) {
					containerInfo = append(containerInfo, &wwapiv1.ContainerInfo{
						Name:          "test",
						NodeCount:     1,
						KernelVersion: "kernel",
						CreateDate:    uint64(time.Unix(0, 0).Unix()),
						ModDate:       uint64(time.Unix(0, 0).Unix()),
						Size:          uint64(1),
					})
					return
				}
			},
		},
	}

	warewulfd.SetNoDaemon()
	for _, tt := range tests {
		env := testenv.New(t)
		env.WriteFile(t, "etc/warewulf/nodes.conf", tt.inDb)

		t.Logf("Running test: %s\n", tt.name)
		t.Run(tt.name, func(t *testing.T) {
			tt.mockFunc()
			buf := new(bytes.Buffer)
			baseCmd := GetCommand()
			baseCmd.SetArgs(tt.args)
			baseCmd.SetOut(nil)
			baseCmd.SetErr(nil)
			wwlog.SetLogWriter(buf)
			err := baseCmd.Execute()
			assert.NoError(t, err)
			assert.Contains(t,
				strings.Join(strings.Fields(tt.stdout), ""),
				strings.Join(strings.Fields(buf.String()), ""))
		})
	}
}
