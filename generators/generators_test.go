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

func TestRestoreFiles(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	// Create test directory
	err := os.MkdirAll(filepath.Join(cacheDir, "rootfs", "testdir1"), 0755)
	require.NoError(t, err)

	// Create original test file
	createTestFile(t, filepath.Join(cacheDir, "rootfs", "testdir1", "testfile1"),
		"original file")

	// Chmod cache directory which should lead to StoreFile failing
	err = os.Chmod(cacheDir, 0600)
	require.NoError(t, err)

	err = StoreFile(cacheDir, rootfsDir, filepath.Join("/testdir1", "testfile1"))
	require.Error(t, err)

	// Restore permissions
	err = os.Chmod(cacheDir, 0755)
	require.NoError(t, err)

	err = StoreFile(cacheDir, rootfsDir, filepath.Join("/testdir1", "testfile1"))
	require.NoError(t, err)

	validateTestFile(t, filepath.Join(cacheDir, "tmp", "testdir1", "testfile1"),
		"original file")

	// Change content of original file
	createTestFile(t, filepath.Join(cacheDir, "rootfs", "testdir1", "testfile1"),
		"modified file")

	err = RestoreFiles(cacheDir, rootfsDir)
	require.NoError(t, err)

	validateTestFile(t, filepath.Join(cacheDir, "rootfs", "testdir1", "testfile1"),
		"original file")
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
