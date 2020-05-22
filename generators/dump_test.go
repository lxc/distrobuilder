package generators

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/distrobuilder/shared"
)

func TestDumpGeneratorRunLXC(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("dump")
	require.Equal(t, DumpGenerator{}, generator)

	err := generator.RunLXC(cacheDir, rootfsDir, nil, shared.DefinitionTargetLXC{
		CreateMessage: "message",
	},
		shared.DefinitionFile{
			Path:    "/hello/world",
			Content: "hello {{ lxc.CreateMessage }}",
			Pongo:   true,
		})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(rootfsDir, "hello", "world"))

	var buffer bytes.Buffer
	file, err := os.Open(filepath.Join(rootfsDir, "hello", "world"))
	require.NoError(t, err)
	defer file.Close()

	io.Copy(&buffer, file)

	require.Equal(t, "hello message\n", buffer.String())

	err = generator.RunLXC(cacheDir, rootfsDir, nil, shared.DefinitionTargetLXC{
		CreateMessage: "message",
	},
		shared.DefinitionFile{
			Path:    "/hello/world",
			Content: "hello {{ lxc.CreateMessage }}",
		})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(rootfsDir, "hello", "world"))

	file, err = os.Open(filepath.Join(rootfsDir, "hello", "world"))
	require.NoError(t, err)
	defer file.Close()

	buffer.Reset()
	io.Copy(&buffer, file)

	require.Equal(t, "hello {{ lxc.CreateMessage }}\n", buffer.String())
}

func TestDumpGeneratorRunLXD(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("dump")
	require.Equal(t, DumpGenerator{}, generator)

	err := generator.RunLXD(cacheDir, rootfsDir, nil, shared.DefinitionTargetLXD{
		VM: shared.DefinitionTargetLXDVM{
			Filesystem: "ext4",
		},
	},
		shared.DefinitionFile{
			Path:    "/hello/world",
			Content: "hello {{ lxd.VM.Filesystem }}",
			Pongo:   true,
		})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(rootfsDir, "hello", "world"))

	var buffer bytes.Buffer
	file, err := os.Open(filepath.Join(rootfsDir, "hello", "world"))
	require.NoError(t, err)
	defer file.Close()

	io.Copy(&buffer, file)

	require.Equal(t, "hello ext4\n", buffer.String())

	file.Close()

	err = generator.RunLXD(cacheDir, rootfsDir, nil, shared.DefinitionTargetLXD{
		VM: shared.DefinitionTargetLXDVM{
			Filesystem: "ext4",
		},
	},
		shared.DefinitionFile{
			Path:    "/hello/world",
			Content: "hello {{ lxd.VM.Filesystem }}",
		})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(rootfsDir, "hello", "world"))

	file, err = os.Open(filepath.Join(rootfsDir, "hello", "world"))
	require.NoError(t, err)
	defer file.Close()

	buffer.Reset()
	io.Copy(&buffer, file)

	require.Equal(t, "hello {{ lxd.VM.Filesystem }}\n", buffer.String())
}
