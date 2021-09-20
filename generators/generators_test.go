package generators

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lxc/distrobuilder/shared"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T, cacheDir string) {
	// Create rootfs directory
	err := os.MkdirAll(filepath.Join(cacheDir, "rootfs"), 0755)
	require.NoError(t, err)
}

func teardown(cacheDir string) {
	os.RemoveAll(cacheDir)
}

func TestGet(t *testing.T) {
	generator, err := Load("hostname", nil, "", "", shared.DefinitionFile{}, shared.Definition{})
	require.IsType(t, &hostname{}, generator)
	require.NoError(t, err)

	generator, err = Load("", nil, "", "", shared.DefinitionFile{}, shared.Definition{})
	require.Nil(t, generator)
	require.Error(t, err)
}

func createTestFile(t *testing.T, path, content string) {
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()

	_, err = file.WriteString(content)
	require.NoError(t, err)
}

func validateTestFile(t *testing.T, path, content string) {
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	var buffer bytes.Buffer
	io.Copy(&buffer, file)

	require.Equal(t, content, buffer.String())
}
