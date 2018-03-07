package generators

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

func TestHostnameGeneratorRunLXC(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("hostname")
	if generator == nil {
		t.Fatal("Expected hostname generator, got nil")
	}

	definition := shared.DefinitionImage{
		Distribution: "ubuntu",
		Release:      "artful",
	}

	image := image.NewLXCImage(cacheDir, "", cacheDir, definition, shared.DefinitionTargetLXC{})

	err := os.MkdirAll(filepath.Join(cacheDir, "rootfs", "etc"), 0755)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	createTestFile(t, filepath.Join(cacheDir, "rootfs", "etc", "hostname"), "hostname")

	err = generator.RunLXC(cacheDir, rootfsDir, image,
		shared.DefinitionFile{Path: "/etc/hostname"})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	validateTestFile(t, filepath.Join(cacheDir, "tmp", "etc", "hostname"), "hostname")
	validateTestFile(t, filepath.Join(cacheDir, "rootfs", "etc", "hostname"), "LXC_NAME\n")

	err = RestoreFiles(cacheDir, rootfsDir)
	if err != nil {
		t.Fatalf("Failed to restore files: %s", err)
	}

	validateTestFile(t, filepath.Join(cacheDir, "rootfs", "etc", "hostname"), "hostname")
}

func TestHostnameGeneratorRunLXD(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("hostname")
	if generator == nil {
		t.Fatal("Expected hostname generator, got nil")
	}

	definition := shared.DefinitionImage{
		Distribution: "ubuntu",
		Release:      "artful",
	}

	image := image.NewLXDImage(cacheDir, "", cacheDir, definition)

	err := os.MkdirAll(filepath.Join(cacheDir, "rootfs", "etc"), 0755)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	err = generator.RunLXD(cacheDir, rootfsDir, image,
		shared.DefinitionFile{Path: "/etc/hostname"})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	validateTestFile(t, filepath.Join(cacheDir, "templates", "hostname.tpl"), "{{ container.name }}\n")
}
