package generators

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

func TestHostsGeneratorCreateLXCData(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("hosts")
	if generator == nil {
		t.Fatal("Expected hosts generator, got nil")
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

	createTestFile(t, filepath.Join(cacheDir, "rootfs", "etc", "hosts"),
		"127.0.0.1\tlocalhost\n")

	err = generator.CreateLXCData(cacheDir, "/etc/hosts", image)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	validateTestFile(t, filepath.Join(cacheDir, "tmp", "etc", "hosts"),
		"127.0.0.1\tlocalhost\n")
	validateTestFile(t, filepath.Join(cacheDir, "rootfs", "etc", "hosts"),
		"127.0.0.1\tlocalhost\n127.0.0.1\tLXC_NAME\n")

	err = RestoreFiles(cacheDir)
	if err != nil {
		t.Fatalf("Failed to restore files: %s", err)
	}

	validateTestFile(t, filepath.Join(cacheDir, "rootfs", "etc", "hosts"),
		"127.0.0.1\tlocalhost\n")
}

func TestHostsGeneratorCreateLXDData(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("hosts")
	if generator == nil {
		t.Fatal("Expected hosts generator, got nil")
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

	createTestFile(t, filepath.Join(cacheDir, "rootfs", "etc", "hosts"),
		"127.0.0.1\tlocalhost\n")

	err = generator.CreateLXDData(cacheDir, "/etc/hosts", image)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	validateTestFile(t, filepath.Join(cacheDir, "templates", "hosts.tpl"),
		"127.0.0.1\tlocalhost\n127.0.0.1\t{{ container.name }}\n")
}
