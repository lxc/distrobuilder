package generators

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

func TestHostnameGeneratorRunLXC(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator, err := Load("hostname", nil, cacheDir, rootfsDir, shared.DefinitionFile{Path: "/etc/hostname"})
	require.IsType(t, &hostname{}, generator)
	require.NoError(t, err)

	definition := shared.Definition{
		Image: shared.DefinitionImage{
			Distribution: "ubuntu",
			Release:      "artful",
		},
	}

	image := image.NewLXCImage(cacheDir, "", cacheDir, definition)

	err = os.MkdirAll(filepath.Join(cacheDir, "rootfs", "etc"), 0755)
	require.NoError(t, err)

	createTestFile(t, filepath.Join(cacheDir, "rootfs", "etc", "hostname"), "hostname")

	err = generator.RunLXC(image, shared.DefinitionTargetLXC{})
	require.NoError(t, err)

	validateTestFile(t, filepath.Join(cacheDir, "rootfs", "etc", "hostname"), "LXC_NAME\n")
}

func TestHostnameGeneratorRunLXD(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator, err := Load("hostname", nil, cacheDir, rootfsDir, shared.DefinitionFile{Path: "/etc/hostname"})
	require.IsType(t, &hostname{}, generator)
	require.NoError(t, err)

	definition := shared.Definition{
		Image: shared.DefinitionImage{
			Distribution: "ubuntu",
			Release:      "artful",
		},
	}

	image := image.NewLXDImage(cacheDir, "", cacheDir, definition)

	err = os.MkdirAll(filepath.Join(cacheDir, "rootfs", "etc"), 0755)
	require.NoError(t, err)

	createTestFile(t, filepath.Join(cacheDir, "rootfs", "etc", "hostname"), "hostname")

	err = generator.RunLXD(image, shared.DefinitionTargetLXD{})
	require.NoError(t, err)

	validateTestFile(t, filepath.Join(cacheDir, "templates", "hostname.tpl"), "{{ container.name }}\n")
}
