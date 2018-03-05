package generators

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func setup(t *testing.T, cacheDir string) {
	// Create rootfs directory
	err := os.MkdirAll(filepath.Join(cacheDir, "rootfs"), 0755)
	if err != nil {
		t.Fatalf("Failed to create rootfs directory: %s", err)
	}
}

func teardown(cacheDir string) {
	os.RemoveAll(cacheDir)
}

func TestGet(t *testing.T) {
	generator := Get("hostname")
	if generator == nil || reflect.DeepEqual(&generator, HostnameGenerator{}) {
		t.Fatal("Expected hostname generator")
	}

	generator = Get("hosts")
	if generator == nil || reflect.DeepEqual(&generator, HostsGenerator{}) {
		t.Fatal("Expected hosts generator")
	}

	generator = Get("")
	if generator != nil {
		t.Fatalf("Expected nil, got '%v'", generator)
	}
}

func TestRestoreFiles(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	// Create test directory
	err := os.MkdirAll(filepath.Join(cacheDir, "rootfs", "testdir1"), 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %s", err)
	}

	// Create original test file
	createTestFile(t, filepath.Join(cacheDir, "rootfs", "testdir1", "testfile1"),
		"original file")

	// Chmod cache directory which should lead to StoreFile failing
	err = os.Chmod(cacheDir, 0600)
	if err != nil {
		t.Fatalf("Failed to chmod cache directory: %s", err)
	}

	err = StoreFile(cacheDir, filepath.Join("/testdir1", "testfile1"))
	if err == nil {
		t.Fatal("Expected failure")
	}

	// Restore permissions
	err = os.Chmod(cacheDir, 0755)
	if err != nil {
		t.Fatalf("Failed to chmod cache directory: %s", err)
	}

	err = StoreFile(cacheDir, filepath.Join("/testdir1", "testfile1"))
	if err != nil {
		t.Fatalf("Failed to store file: %s", err)
	}

	validateTestFile(t, filepath.Join(cacheDir, "tmp", "testdir1", "testfile1"),
		"original file")

	// Change content of original file
	createTestFile(t, filepath.Join(cacheDir, "rootfs", "testdir1", "testfile1"),
		"modified file")

	err = RestoreFiles(cacheDir)
	if err != nil {
		t.Fatalf("Failed to restore file: %s", err)
	}

	validateTestFile(t, filepath.Join(cacheDir, "rootfs", "testdir1", "testfile1"),
		"original file")
}

func createTestFile(t *testing.T, path, content string) {
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		t.Fatalf("Failed to write to testfile")
	}
}

func validateTestFile(t *testing.T, path, content string) {
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open testfile: %s", err)
	}
	defer file.Close()

	var buffer bytes.Buffer
	io.Copy(&buffer, file)

	if buffer.String() != content {
		t.Fatalf("Expected file content to be '%s', got '%s'", content, buffer.String())
	}
}
