package generators

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

func TestTemplateGeneratorRunLXD(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("template")
	require.Equal(t, TemplateGenerator{}, generator)

	definition := shared.Definition{
		Image: shared.DefinitionImage{
			Distribution: "ubuntu",
			Release:      "artful",
		},
	}

	image := image.NewLXDImage(cacheDir, "", cacheDir, definition)

	err := os.MkdirAll(filepath.Join(cacheDir, "rootfs", "root"), 0755)
	require.NoError(t, err)

	createTestFile(t, filepath.Join(cacheDir, "rootfs", "root", "template"), "--test--")

	err = generator.RunLXD(cacheDir, rootfsDir, image, shared.DefinitionFile{
		Generator: "template",
		Name:      "template",
		Content:   "==test==",
		Path:      "/root/template",
	})
	require.NoError(t, err)

	validateTestFile(t, filepath.Join(cacheDir, "templates", "template.tpl"), "==test==\n")
	validateTestFile(t, filepath.Join(cacheDir, "rootfs", "root", "template"), "--test--")
}

func TestTemplateGeneratorRunLXDDefaultWhen(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("template")
	require.Equal(t, TemplateGenerator{}, generator)

	definition := shared.Definition{
		Image: shared.DefinitionImage{
			Distribution: "ubuntu",
			Release:      "artful",
		},
	}

	image := image.NewLXDImage(cacheDir, "", cacheDir, definition)

	err := generator.RunLXD(cacheDir, rootfsDir, image, shared.DefinitionFile{
		Generator: "template",
		Name:      "test-default-when",
		Content:   "==test==",
		Path:      "test-default-when",
	})
	require.NoError(t, err)

	err = generator.RunLXD(cacheDir, rootfsDir, image, shared.DefinitionFile{
		Generator: "template",
		Name:      "test-when",
		Content:   "==test==",
		Path:      "test-when",
		Template: shared.DefinitionFileTemplate{
			When: []string{"create"},
		},
	})
	require.NoError(t, err)

	testvalue := []string{"create", "copy"}
	require.Equal(t, image.Metadata.Templates["test-default-when"].When, testvalue)

	testvalue = []string{"create"}
	require.Equal(t, image.Metadata.Templates["test-when"].When, testvalue)
}
