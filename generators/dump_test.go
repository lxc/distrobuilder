package generators

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

func TestDumpGeneratorRunLXC(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("dump")
	if generator == nil {
		t.Fatal("Expected dump generator, got nil")
	}

	err := generator.RunLXC(cacheDir, rootfsDir, nil,
		shared.DefinitionFile{
			Path:    "/hello/world",
			Content: "hello world",
		})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if !lxd.PathExists(filepath.Join(rootfsDir, "hello", "world")) {
		t.Fatalf("Directory '%s' wasn't created", "/hello/world")
	}

	var buffer bytes.Buffer
	file, err := os.Open(filepath.Join(rootfsDir, "hello", "world"))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer file.Close()

	io.Copy(&buffer, file)

	if buffer.String() != "hello world" {
		t.Fatalf("Expected '%s', got '%s'", "hello world", buffer.String())
	}
}

func TestDumpGeneratorRunLXD(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("dump")
	if generator == nil {
		t.Fatal("Expected dump generator, got nil")
	}

	err := generator.RunLXD(cacheDir, rootfsDir, nil,
		shared.DefinitionFile{
			Path:    "/hello/world",
			Content: "hello world",
		})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if !lxd.PathExists(filepath.Join(rootfsDir, "hello", "world")) {
		t.Fatalf("Directory '%s' wasn't created", "/hello/world")
	}

	var buffer bytes.Buffer
	file, err := os.Open(filepath.Join(rootfsDir, "hello", "world"))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer file.Close()

	io.Copy(&buffer, file)

	if buffer.String() != "hello world" {
		t.Fatalf("Expected '%s', got '%s'", "hello world", buffer.String())
	}
}
