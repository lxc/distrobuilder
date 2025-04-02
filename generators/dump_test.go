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
	cacheDir, err := os.MkdirTemp(os.TempDir(), "distrobuilder-test-")
	require.NoError(t, err)

	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	def := shared.Definition{
		Targets: shared.DefinitionTarget{
			LXC: shared.DefinitionTargetLXC{
				CreateMessage: "message",
			},
		},
	}

	generator, err := Load("dump", nil, cacheDir, rootfsDir, shared.DefinitionFile{
		Path:    "/hello/world",
		Content: "hello {{ targets.lxc.create_message }}",
		Pongo:   true,
	}, def)
	require.IsType(t, &dump{}, generator)
	require.NoError(t, err)

	err = generator.RunLXC(nil, shared.DefinitionTargetLXC{
		CreateMessage: "message",
	})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(rootfsDir, "hello", "world"))

	var buffer bytes.Buffer
	file, err := os.Open(filepath.Join(rootfsDir, "hello", "world"))
	require.NoError(t, err)
	defer file.Close()

	_, err = io.Copy(&buffer, file)
	require.NoError(t, err)

	require.Equal(t, "hello message\n", buffer.String())

	generator, err = Load("dump", nil, cacheDir, rootfsDir, shared.DefinitionFile{
		Path:    "/hello/world",
		Content: "hello {{ targets.lxc.create_message }}",
	}, def)
	require.IsType(t, &dump{}, generator)
	require.NoError(t, err)

	err = generator.RunLXC(nil, shared.DefinitionTargetLXC{
		CreateMessage: "message",
	})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(rootfsDir, "hello", "world"))

	file, err = os.Open(filepath.Join(rootfsDir, "hello", "world"))
	require.NoError(t, err)
	defer file.Close()

	buffer.Reset()
	_, err = io.Copy(&buffer, file)
	require.NoError(t, err)

	require.Equal(t, "hello {{ targets.lxc.create_message }}\n", buffer.String())
}

func TestDumpGeneratorRunIncus(t *testing.T) {
	cacheDir, err := os.MkdirTemp(os.TempDir(), "distrobuilder-test-")
	require.NoError(t, err)

	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	def := shared.Definition{
		Targets: shared.DefinitionTarget{
			Incus: shared.DefinitionTargetIncus{
				VM: shared.DefinitionTargetIncusVM{
					Filesystem: "ext4",
				},
			},
		},
	}

	generator, err := Load("dump", nil, cacheDir, rootfsDir, shared.DefinitionFile{
		Path:    "/hello/world",
		Content: "hello {{ targets.incus.vm.filesystem }}",
		Pongo:   true,
	}, def)
	require.IsType(t, &dump{}, generator)
	require.NoError(t, err)

	err = generator.RunIncus(nil, shared.DefinitionTargetIncus{
		VM: shared.DefinitionTargetIncusVM{
			Filesystem: "ext4",
		},
	})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(rootfsDir, "hello", "world"))

	var buffer bytes.Buffer
	file, err := os.Open(filepath.Join(rootfsDir, "hello", "world"))
	require.NoError(t, err)
	defer file.Close()

	_, err = io.Copy(&buffer, file)
	require.NoError(t, err)

	require.Equal(t, "hello ext4\n", buffer.String())

	file.Close()

	generator, err = Load("dump", nil, cacheDir, rootfsDir, shared.DefinitionFile{
		Path:    "/hello/world",
		Content: "hello {{ targets.incus.vm.filesystem }}",
	}, def)
	require.IsType(t, &dump{}, generator)
	require.NoError(t, err)

	err = generator.RunIncus(nil, shared.DefinitionTargetIncus{
		VM: shared.DefinitionTargetIncusVM{
			Filesystem: "ext4",
		},
	})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(rootfsDir, "hello", "world"))

	file, err = os.Open(filepath.Join(rootfsDir, "hello", "world"))
	require.NoError(t, err)
	defer file.Close()

	buffer.Reset()
	_, err = io.Copy(&buffer, file)
	require.NoError(t, err)

	require.Equal(t, "hello {{ targets.incus.vm.filesystem }}\n", buffer.String())
}
