package image

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/lxc/distrobuilder/shared"
)

var incusDef = shared.Definition{
	Image: shared.DefinitionImage{
		Description:  "{{ image.distribution|capfirst }} {{ image. release }}",
		Distribution: "ubuntu",
		Release:      "17.10",
		Architecture: "x86_64",
		Expiry:       "30d",
		Name:         "{{ image.distribution|lower }}-{{ image.release }}-{{ image.architecture }}-{{ image.serial }}",
		Serial:       "testing",
	},
	Source: shared.DefinitionSource{
		Downloader: "debootstrap",
	},
	Packages: shared.DefinitionPackages{
		Manager: "apt",
	},
}

func setupIncus(t *testing.T) *IncusImage {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")

	err := os.MkdirAll(filepath.Join(cacheDir, "rootfs"), 0755)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(cacheDir, "templates"), 0755)
	require.NoError(t, err)

	image := NewIncusImage(context.TODO(), cacheDir, "", cacheDir, incusDef)

	fail := true
	defer func() {
		if fail {
			teardownIncus(t)
		}
	}()

	// Check cache directory
	require.Equal(t, cacheDir, image.cacheDir)
	require.Equal(t, incusDef, image.definition)

	incusDef.SetDefaults()

	err = incusDef.Validate()
	require.NoError(t, err)

	fail = false
	return image
}

func teardownIncus(t *testing.T) {
	os.RemoveAll(filepath.Join(os.TempDir(), "distrobuilder-test"))
}

func TestIncusBuild(t *testing.T) {
	image := setupIncus(t)
	defer teardownIncus(t)

	testIncusBuildSplitImage(t, image)
	testIncusBuildUnifiedImage(t, image)
}

func testIncusBuildSplitImage(t *testing.T, image *IncusImage) {
	// Create split tarball and squashfs.
	imageFile, rootfsFile, err := image.Build(false, "xz", false)
	require.NoError(t, err)
	require.FileExists(t, "incus.tar.xz")
	require.FileExists(t, "rootfs.squashfs")
	require.Equal(t, "rootfs.squashfs", filepath.Base(rootfsFile))
	require.Equal(t, "incus.tar.xz", filepath.Base(imageFile))

	os.Remove("incus.tar.xz")
	os.Remove("rootfs.squashfs")

	imageFile, rootfsFile, err = image.Build(false, "gzip", false)
	require.NoError(t, err)
	require.FileExists(t, "incus.tar.gz")
	require.FileExists(t, "rootfs.squashfs")
	require.Equal(t, "rootfs.squashfs", filepath.Base(rootfsFile))
	require.Equal(t, "incus.tar.gz", filepath.Base(imageFile))

	os.Remove("incus.tar.gz")
	os.Remove("rootfs.squashfs")
}

func testIncusBuildUnifiedImage(t *testing.T, image *IncusImage) {
	// Create unified tarball with custom name.
	_, _, err := image.Build(true, "xz", false)
	require.NoError(t, err)
	defer os.Remove("ubuntu-17.10-x86_64-testing.tar.xz")

	require.FileExists(t, "ubuntu-17.10-x86_64-testing.tar.xz")

	_, _, err = image.Build(true, "gzip", false)
	require.NoError(t, err)
	defer os.Remove("ubuntu-17.10-x86_64-testing.tar.gz")

	require.FileExists(t, "ubuntu-17.10-x86_64-testing.tar.gz")

	// Create unified tarball with default name.
	image.definition.Image.Name = ""
	_, _, err = image.Build(true, "xz", false)
	require.NoError(t, err)
	defer os.Remove("incus.tar.xz")

	require.FileExists(t, "incus.tar.xz")
}

func TestIncusCreateMetadata(t *testing.T) {
	image := setupIncus(t)
	defer teardownIncus(t)

	err := image.createMetadata()
	require.NoError(t, err)

	tests := []struct {
		name     string
		have     string
		expected string
	}{
		{
			"Architecture",
			image.Metadata.Architecture,
			"x86_64",
		},
		{
			"Properties[architecture]",
			image.Metadata.Properties["architecture"],
			"x86_64",
		},
		{
			"Properties[os]",
			image.Metadata.Properties["os"],
			incusDef.Image.Distribution,
		},
		{
			"Properties[release]",
			image.Metadata.Properties["release"],
			incusDef.Image.Release,
		},
		{
			"Properties[description]",
			image.Metadata.Properties["description"],
			fmt.Sprintf("%s %s", cases.Title(language.English).String(incusDef.Image.Distribution),
				incusDef.Image.Release),
		},
		{
			"Properties[name]",
			image.Metadata.Properties["name"],
			fmt.Sprintf("%s-%s-%s-%s", strings.ToLower(incusDef.Image.Distribution),
				incusDef.Image.Release, "x86_64", incusDef.Image.Serial),
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		require.Equal(t, tt.expected, tt.have)
	}
}
