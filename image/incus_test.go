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

var lxdDef = shared.Definition{
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

func setupLXD(t *testing.T) *LXDImage {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")

	err := os.MkdirAll(filepath.Join(cacheDir, "rootfs"), 0755)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(cacheDir, "templates"), 0755)
	require.NoError(t, err)

	image := NewLXDImage(context.TODO(), cacheDir, "", cacheDir, lxdDef)

	fail := true
	defer func() {
		if fail {
			teardownLXD(t)
		}
	}()

	// Check cache directory
	require.Equal(t, cacheDir, image.cacheDir)
	require.Equal(t, lxdDef, image.definition)

	lxdDef.SetDefaults()

	err = lxdDef.Validate()
	require.NoError(t, err)

	fail = false
	return image
}

func teardownLXD(t *testing.T) {
	os.RemoveAll(filepath.Join(os.TempDir(), "distrobuilder-test"))
}

func TestLXDBuild(t *testing.T) {
	image := setupLXD(t)
	defer teardownLXD(t)

	testLXDBuildSplitImage(t, image)
	testLXDBuildUnifiedImage(t, image)
}

func testLXDBuildSplitImage(t *testing.T, image *LXDImage) {
	// Create split tarball and squashfs.
	imageFile, rootfsFile, err := image.Build(false, "xz", false)
	require.NoError(t, err)
	require.FileExists(t, "lxd.tar.xz")
	require.FileExists(t, "rootfs.squashfs")
	require.Equal(t, "rootfs.squashfs", filepath.Base(rootfsFile))
	require.Equal(t, "lxd.tar.xz", filepath.Base(imageFile))

	os.Remove("lxd.tar.xz")
	os.Remove("rootfs.squashfs")

	imageFile, rootfsFile, err = image.Build(false, "gzip", false)
	require.NoError(t, err)
	require.FileExists(t, "lxd.tar.gz")
	require.FileExists(t, "rootfs.squashfs")
	require.Equal(t, "rootfs.squashfs", filepath.Base(rootfsFile))
	require.Equal(t, "lxd.tar.gz", filepath.Base(imageFile))

	os.Remove("lxd.tar.gz")
	os.Remove("rootfs.squashfs")
}

func testLXDBuildUnifiedImage(t *testing.T, image *LXDImage) {
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
	defer os.Remove("lxd.tar.xz")

	require.FileExists(t, "lxd.tar.xz")
}

func TestLXDCreateMetadata(t *testing.T) {
	image := setupLXD(t)
	defer teardownLXD(t)

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
			lxdDef.Image.Distribution,
		},
		{
			"Properties[release]",
			image.Metadata.Properties["release"],
			lxdDef.Image.Release,
		},
		{
			"Properties[description]",
			image.Metadata.Properties["description"],
			fmt.Sprintf("%s %s", cases.Title(language.English).String(lxdDef.Image.Distribution),
				lxdDef.Image.Release),
		},
		{
			"Properties[name]",
			image.Metadata.Properties["name"],
			fmt.Sprintf("%s-%s-%s-%s", strings.ToLower(lxdDef.Image.Distribution),
				lxdDef.Image.Release, "x86_64", lxdDef.Image.Serial),
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		require.Equal(t, tt.expected, tt.have)
	}
}
