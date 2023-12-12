package overlay

import (
	"io"
	"os"
	"path"
	"sort"
	"testing"

	warewulfconf "github.com/hpcng/warewulf/internal/pkg/config"
	"github.com/hpcng/warewulf/internal/pkg/node"
	"github.com/sassoftware/go-rpmutils/cpio"
	"github.com/stretchr/testify/assert"
)

var buildOverlayTests = []struct {
	description string
	nodeName    string
	context     string
	overlays    []string
	image       string
	contents    []string
	hasFiles    bool
}{
	{
		description: "if no node, context, or overlays are specified then no overlay image is generated",
		nodeName:    "",
		context:     "",
		overlays:    nil,
		image:       "",
		contents:    nil,
	},
	{
		description: "if only node is specified then no overlay image is generated",
		nodeName:    "node1",
		context:     "",
		overlays:    nil,
		image:       "",
		contents:    nil,
	},
	{
		description: "if only context is specified then no overlay image is generated",
		nodeName:    "",
		context:     "system",
		overlays:    nil,
		image:       "",
		contents:    nil,
	},
	{
		description: "if an overlay is specified without a node, then the overlay is built directly in the overlay directory",
		nodeName:    "",
		context:     "",
		overlays:    []string{"o1"},
		image:       "o1.img",
		contents:    []string{"o1.txt"},
	},
	{
		description: "if multiple overlays are specified without a node, then the combined overlay is built directly in the overlay directory",
		nodeName:    "",
		context:     "",
		overlays:    []string{"o1", "o2"},
		image:       "o1-o2.img",
		contents:    []string{"o1.txt", "o2.txt"},
	},
	{
		description: "if a single node overlay is specified, then the overlay is built in a node overlay directory",
		nodeName:    "node1",
		context:     "",
		overlays:    []string{"o1"},
		image:       "node1/o1.img",
		contents:    []string{"o1.txt"},
	},
	{
		description: "if multiple node overlays are specified, then the combined overlay is built in a node overlay directory",
		nodeName:    "node1",
		context:     "",
		overlays:    []string{"o1", "o2"},
		image:       "node1/o1-o2.img",
		contents:    []string{"o1.txt", "o2.txt"},
	},
	{
		description: "if no node system overlays are specified, then no overlay image is generated",
		nodeName:    "node1",
		context:     "system",
		overlays:    nil,
		image:       "",
		contents:    nil,
	},
	{
		description: "if no node runtime overlays are specified, then no overlay image is generated",
		nodeName:    "node1",
		context:     "runtime",
		overlays:    nil,
		image:       "",
		contents:    nil,
	},
	{
		description: "if a single node system overlay is specified, then a system overlay image is generated in a node overlay directory",
		nodeName:    "node1",
		context:     "system",
		overlays:    []string{"o1"},
		image:       "node1/__SYSTEM__.img",
		contents:    []string{"o1.txt"},
	},
	{
		description: "if a single node runtime overlay is specified, then a runtime overlay image is generated in a node overlay directory",
		nodeName:    "node1",
		context:     "runtime",
		overlays:    []string{"o1"},
		image:       "node1/__RUNTIME__.img",
		contents:    []string{"o1.txt"},
	},
	{
		description: "if multiple node system overlays are specified, then a system overlay image is generated with the contents of both overlays",
		nodeName:    "node1",
		context:     "system",
		overlays:    []string{"o1", "o2"},
		image:       "node1/__SYSTEM__.img",
		contents:    []string{"o1.txt", "o2.txt"},
	},
	{
		description: "if multiple node runtime overlays are specified, then a runtime overlay image is generated with the contents of both overlays",
		nodeName:    "node1",
		context:     "runtime",
		overlays:    []string{"o1", "o2"},
		image:       "node1/__RUNTIME__.img",
		contents:    []string{"o1.txt", "o2.txt"},
	},
}

func Test_BuildOverlay(t *testing.T) {
	conf := warewulfconf.Get()
	overlayDir, overlayDirErr := os.MkdirTemp(os.TempDir(), "ww-test-overlay-*")
	assert.NoError(t, overlayDirErr)
	defer os.RemoveAll(overlayDir)
	conf.Paths.WWOverlaydir = overlayDir
	assert.NoError(t, os.Mkdir(path.Join(overlayDir, "o1"), 0700))
	{
		_, err := os.Create(path.Join(overlayDir, "o1", "o1.txt"))
		assert.NoError(t, err)
	}
	assert.NoError(t, os.Mkdir(path.Join(overlayDir, "o2"), 0700))
	{
		_, err := os.Create(path.Join(overlayDir, "o2", "o2.txt"))
		assert.NoError(t, err)
	}

	for _, tt := range buildOverlayTests {
		assert.True(t, (tt.image != "" && tt.contents != nil) || (tt.image == "" && tt.contents == nil),
			"image and contents must eiher be populated or empty together")

		nodeInfo := node.NewConf(tt.nodeName)
		t.Run(tt.description, func(t *testing.T) {
			provisionDir, provisionDirErr := os.MkdirTemp(os.TempDir(), "ww-test-provision-*")
			assert.NoError(t, provisionDirErr)
			defer os.RemoveAll(provisionDir)
			conf.Paths.WWProvisiondir = provisionDir

			err := BuildOverlay(nodeInfo.Id(), tt.context, tt.overlays)
			assert.NoError(t, err)
			if tt.image != "" {
				image := path.Join(provisionDir, "overlays", tt.image)
				assert.FileExists(t, image)

				sort.Strings(tt.contents)
				files := cpioFiles(t, image)
				sort.Strings(files)
				assert.Equal(t, tt.contents, files)
			} else {
				dirName := path.Join(provisionDir, "overlays", tt.nodeName)
				isEmpty := dirIsEmpty(t, dirName)
				assert.True(t, isEmpty, "%v should be empty, but isn't", dirName)
			}
		})
	}
}

// Although these tests specify system and runtime overlays for the
// nodes, these overlays define the overlays that are defined in the
// configuration. BuildAllOverlays doesn't receive these as arguments,
// but builds all the overlays that are configured on the given node.
var buildAllOverlaysTests = []struct {
	description     string
	nodes           []string
	systemOverlays  [][]string
	runtimeOverlays [][]string
	createdOverlays []string
}{
	{
		description:     "empty input creates no overlays",
		nodes:           nil,
		systemOverlays:  nil,
		runtimeOverlays: nil,
		createdOverlays: nil,
	},
	{
		description:     "a node with no overlays creates no overlays",
		nodes:           []string{"node1"},
		systemOverlays:  nil,
		runtimeOverlays: nil,
		createdOverlays: nil,
	},
	{
		description:     "multiple nodes with no overlays creates no overlays",
		nodes:           []string{"node1", "node2"},
		systemOverlays:  nil,
		runtimeOverlays: nil,
		createdOverlays: nil,
	},
	{
		description:     "a system overlay for a node generates a system overlay for that node",
		nodes:           []string{"node1"},
		systemOverlays:  [][]string{{"o1"}},
		runtimeOverlays: nil,
		createdOverlays: []string{"node1/__SYSTEM__.img.gz"},
	},
	{
		description:     "two nodes with different system overlays generates a system overlay for each node",
		nodes:           []string{"node1", "node2"},
		systemOverlays:  [][]string{{"o1"}, {"o1", "o2"}},
		runtimeOverlays: nil,
		createdOverlays: []string{"node1/__SYSTEM__.img.gz", "node2/__SYSTEM__.img.gz"},
	},
	{
		description:     "two nodes with a single runtime overlay generates a runtime overlay for the first node",
		nodes:           []string{"node1"},
		systemOverlays:  nil,
		runtimeOverlays: [][]string{{"o1"}},
		createdOverlays: []string{"node1/__RUNTIME__.img.gz"},
	},
	{
		description:     "two nodes with different runtime overlays generates a system overlay for each node",
		nodes:           []string{"node1", "node2"},
		systemOverlays:  nil,
		runtimeOverlays: [][]string{{"o1"}, {"o1", "o2"}},
		createdOverlays: []string{"node1/__RUNTIME__.img.gz", "node2/__RUNTIME__.img.gz"},
	},
	{
		description:     "a node with both a runtime and system overlay generates an image for each",
		nodes:           []string{"node1"},
		systemOverlays:  [][]string{{"o1"}},
		runtimeOverlays: [][]string{{"o2"}},
		createdOverlays: []string{"node1/__RUNTIME__.img.gz", "node1/__SYSTEM__.img.gz"},
	},
	{
		description:     "two nodes with both runtime and system overlays generates each image for each node",
		nodes:           []string{"node1", "node2"},
		systemOverlays:  [][]string{{"o1"}, {"o1", "o2"}},
		runtimeOverlays: [][]string{{"o2"}, {"o2"}},
		createdOverlays: []string{"node1/__RUNTIME__.img.gz", "node1/__SYSTEM__.img.gz", "node2/__RUNTIME__.img.gz", "node2/__SYSTEM__.img.gz"},
	},
}

func Test_BuildAllOverlays(t *testing.T) {
	conf := warewulfconf.Get()
	overlayDir, overlayDirErr := os.MkdirTemp(os.TempDir(), "ww-test-overlay-*")
	assert.NoError(t, overlayDirErr)
	defer os.RemoveAll(overlayDir)
	conf.Paths.WWOverlaydir = overlayDir
	assert.NoError(t, os.Mkdir(path.Join(overlayDir, "o1"), 0700))
	assert.NoError(t, os.Mkdir(path.Join(overlayDir, "o2"), 0700))

	for _, tt := range buildAllOverlaysTests {
		t.Run(tt.description, func(t *testing.T) {
			provisionDir, provisionDirErr := os.MkdirTemp(os.TempDir(), "ww-test-provision-*")
			assert.NoError(t, provisionDirErr)
			defer os.RemoveAll(provisionDir)
			conf.Paths.WWProvisiondir = provisionDir

			var nodes []node.NodeConf
			for i, nodeName := range tt.nodes {
				nodeInfo := node.NewConf(nodeName)
				if tt.systemOverlays != nil {
					nodeInfo.SystemOverlay = tt.systemOverlays[i]
				}
				if tt.runtimeOverlays != nil {
					nodeInfo.RuntimeOverlay = tt.runtimeOverlays[i]
				}
				nodes = append(nodes, nodeInfo)
			}
			err := BuildAllOverlays(nodes)
			assert.NoError(t, err)
			if tt.createdOverlays == nil {
				dirName := path.Join(provisionDir, "overlays")
				assert.True(t, dirIsEmpty(t, dirName), "%v should be empty, but isn't", dirName)
			}
			for _, overlayPath := range tt.createdOverlays {
				assert.FileExists(t, path.Join(provisionDir, "overlays", overlayPath))
			}
		})
	}
}

var buildSpecificOverlaysTests = []struct {
	description string
	nodes       []string
	overlays    []string
	images      []string
	succeed     bool
}{
	{
		description: "building no overlays for no nodes generates no error and no images",
		nodes:       nil,
		overlays:    nil,
		images:      nil,
		succeed:     true,
	},
	{
		description: "building no overlays for a node generates no error and no images",
		nodes:       []string{"node1"},
		overlays:    nil,
		images:      nil,
		succeed:     true,
	},
	{
		description: "building no overlays for two nodes generates no error and no images",
		nodes:       []string{"node1", "node2"},
		overlays:    nil,
		images:      nil,
		succeed:     true,
	},
	{
		description: "building an overlay for a node generates an overlay image in that node's overlay directory",
		nodes:       []string{"node1"},
		overlays:    []string{"o1"},
		images:      []string{"node1/o1.img"},
		succeed:     true,
	},
	{
		description: "building an overlay for two nodes generates an overlay image in each node's overlay directory",
		nodes:       []string{"node1", "node2"},
		overlays:    []string{"o1"},
		images:      []string{"node1/o1.img", "node2/o1.img"},
		succeed:     true,
	},
	{
		description: "building multiple overlays for a node generates an overlay image for each overlay in the node's overlay directory",
		nodes:       []string{"node1"},
		overlays:    []string{"o1", "o2"},
		images:      []string{"node1/o1.img", "node1/o2.img"},
		succeed:     true,
	},
	{
		description: "building multiple overlays for two nodes generates an overlay image for each overlay in each node's overlay directory",
		nodes:       []string{"node1", "node2"},
		overlays:    []string{"o1", "o2"},
		images:      []string{"node1/o1.img", "node1/o2.img", "node2/o1.img", "node2/o2.img"},
		succeed:     true,
	},
}

func Test_BuildSpecificOverlays(t *testing.T) {
	conf := warewulfconf.Get()
	overlayDir, overlayDirErr := os.MkdirTemp(os.TempDir(), "ww-test-overlay-*")
	assert.NoError(t, overlayDirErr)
	defer os.RemoveAll(overlayDir)
	conf.Paths.WWOverlaydir = overlayDir
	assert.NoError(t, os.Mkdir(path.Join(overlayDir, "o1"), 0700))
	assert.NoError(t, os.Mkdir(path.Join(overlayDir, "o2"), 0700))

	for _, tt := range buildSpecificOverlaysTests {
		t.Run(tt.description, func(t *testing.T) {
			provisionDir, provisionDirErr := os.MkdirTemp(os.TempDir(), "ww-test-provision-*")
			assert.NoError(t, provisionDirErr)
			defer os.RemoveAll(provisionDir)
			conf.Paths.WWProvisiondir = provisionDir
			err := BuildSpecificOverlays(tt.nodes, tt.overlays)
			if !tt.succeed {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for _, image := range tt.images {
					assert.FileExists(t, path.Join(provisionDir, "overlays", image))
				}
			}
		})
	}
}

func dirIsEmpty(t *testing.T, name string) bool {
	f, err := os.Open(name)
	if err != nil {
		t.Log(err)
		return true
	}
	defer f.Close()

	dirnames, err2 := f.Readdirnames(1)
	if err2 == io.EOF {
		t.Log(err2)
		return true
	}
	t.Log(dirnames)
	return false
}

func cpioFiles(t *testing.T, name string) (files []string) {
	f, openErr := os.Open(name)
	if openErr != nil {
		return
	}
	defer f.Close()

	reader := cpio.NewReader(f)
	for {
		header, err := reader.Next()
		if err != nil {
			return
		}
		files = append(files, header.Filename())
	}
}
