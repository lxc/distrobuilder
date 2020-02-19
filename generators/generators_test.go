package generators

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

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
	generator := Get("hostname")
	require.Equal(t, HostnameGenerator{}, generator)

	generator = Get("hosts")
	require.Equal(t, HostsGenerator{}, generator)

	generator = Get("")
	require.Nil(t, generator)
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
