package generators

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

func TestTemplateGeneratorRunLXD(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("template")
	if generator == nil {
		t.Fatal("Expected template generator, got nil")
	}

	definition := shared.Definition{
		Image: shared.DefinitionImage{
			Distribution: "ubuntu",
			Release:      "artful",
		},
	}

	image := image.NewLXDImage(cacheDir, "", cacheDir, definition)

	err := os.MkdirAll(filepath.Join(cacheDir, "rootfs", "root"), 0755)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	createTestFile(t, filepath.Join(cacheDir, "rootfs", "root", "template"), "--test--")

	err = generator.RunLXD(cacheDir, rootfsDir, image, shared.DefinitionFile{
		Generator: "template",
		Name:      "template",
		Content:   "==test==",
		Path:      "/root/template",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	validateTestFile(t, filepath.Join(cacheDir, "templates", "template.tpl"), "==test==")
	validateTestFile(t, filepath.Join(cacheDir, "rootfs", "root", "template"), "--test--")
}
