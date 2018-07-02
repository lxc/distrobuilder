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

	err := generator.RunLXC(cacheDir, rootfsDir, nil,
		shared.DefinitionFile{
			Path:    "/hello/world",
			Content: "hello world",
		})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(rootfsDir, "hello", "world"))

	var buffer bytes.Buffer
	file, err := os.Open(filepath.Join(rootfsDir, "hello", "world"))
	require.NoError(t, err)
	defer file.Close()

	io.Copy(&buffer, file)

	require.Equal(t, "hello world\n", buffer.String())
}

func TestDumpGeneratorRunLXD(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("dump")
	require.Equal(t, DumpGenerator{}, generator)

	err := generator.RunLXD(cacheDir, rootfsDir, nil,
		shared.DefinitionFile{
			Path:    "/hello/world",
			Content: "hello world",
		})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(rootfsDir, "hello", "world"))

	var buffer bytes.Buffer
	file, err := os.Open(filepath.Join(rootfsDir, "hello", "world"))
	require.NoError(t, err)
	defer file.Close()

	io.Copy(&buffer, file)

	require.Equal(t, "hello world\n", buffer.String())
}
