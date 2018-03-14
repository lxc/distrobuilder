package image

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

var lxdDef = shared.Definition{
	Image: shared.DefinitionImage{
		Description:  "{{ image.distribution|capfirst }} {{ image. release }}",
		Distribution: "ubuntu",
		Release:      "17.10",
		Architecture: "amd64",
		Expiry:       "30d",
		Name:         "{{ image.distribution|lower }}-{{ image.release }}-{{ image.architecture }}-{{ image.serial }}",
		Serial:       "testing",
	},
}

func setupLXD(t *testing.T) *LXDImage {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")

	err := os.MkdirAll(filepath.Join(cacheDir, "rootfs"), 0755)
	if err != nil {
		t.Fatalf("Failed to create rootfs directory: %s", err)
	}

	err = os.MkdirAll(filepath.Join(cacheDir, "templates"), 0755)
	if err != nil {
		t.Fatalf("Failed to create templates directory: %s", err)
	}

	image := NewLXDImage(cacheDir, "", cacheDir, lxdDef)

	// Check cache directory
	if image.cacheDir != cacheDir {
		teardownLXD(t)
		t.Fatalf("Expected cacheDir to be '%s', is '%s'", cacheDir, image.cacheDir)
	}

	if !reflect.DeepEqual(lxdDef, image.definition) {
		teardownLXD(t)
		t.Fatal("lxdDef and image.definition are not equal")
	}

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
	err := image.Build(false, "xz")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer func() {
		os.Remove("lxd.tar.xz")
		os.Remove("rootfs.squashfs")
	}()

	if !lxd.PathExists("lxd.tar.xz") {
		t.Fatalf("File '%s' does not exist", "lxd.tar.xz")
	}

	if !lxd.PathExists("rootfs.squashfs") {
		t.Fatalf("File '%s' does not exist", "rootfs.squashfs")
	}
}

func testLXDBuildUnifiedImage(t *testing.T, image *LXDImage) {
	// Create unified tarball with custom name.
	err := image.Build(true, "xz")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer os.Remove("ubuntu-17.10-x86_64-testing.tar.xz")

	if !lxd.PathExists("ubuntu-17.10-x86_64-testing.tar.xz") {
		t.Fatalf("File '%s' does not exist", "ubuntu-17.10-x86_64-testing.tar.xz")
	}

	// Create unified tarball with default name.
	image.definition.Image.Name = ""
	err = image.Build(true, "xz")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer os.Remove("lxd.tar.xz")

	if !lxd.PathExists("lxd.tar.xz") {
		t.Fatalf("File '%s' does not exist", "lxd.tar.xz")
	}
}

func TestLXDCreateMetadata(t *testing.T) {
	image := setupLXD(t)
	defer teardownLXD(t)

	err := image.createMetadata()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

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
			fmt.Sprintf("%s %s", strings.Title(lxdDef.Image.Distribution),
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
		if tt.have != tt.expected {
			t.Fatalf("Expected '%s', got '%s'", tt.expected, tt.have)
		}
	}
}
