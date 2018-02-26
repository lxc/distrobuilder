package image

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
)

var lxdImageDef = shared.DefinitionImage{
	Description:  "{{ image. Distribution|capfirst }} {{ image.Release }}",
	Distribution: "ubuntu",
	Release:      "17.10",
	Arch:         "amd64",
	Expiry:       "30d",
	Name:         "{{ image.Distribution|lower }}-{{ image.Release }}-{{ image.Arch }}-{{ creation_date }}",
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

	image := NewLXDImage(cacheDir, lxdImageDef)

	// Override creation date
	image.creationDate = time.Date(2006, 1, 2, 3, 4, 5, 0, time.UTC)

	// Check cache directory
	if image.cacheDir != cacheDir {
		teardownLXD(t)
		t.Fatalf("Expected cacheDir to be '%s', is '%s'", cacheDir, image.cacheDir)
	}

	if !reflect.DeepEqual(lxdImageDef, image.definition) {
		teardownLXD(t)
		t.Fatal("lxdImageDef and image.definition are not equal")
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
	err := image.Build(false)
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
	err := image.Build(true)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer os.Remove("ubuntu-17.10-x86_64-20060201_0304.tar.xz")

	if !lxd.PathExists("ubuntu-17.10-x86_64-20060201_0304.tar.xz") {
		t.Fatalf("File '%s' does not exist", "ubuntu-17.10-x86_64-20060201_0304.tar.xz")
	}

	// Create unified tarball with default name.
	image.definition.Name = ""
	err = image.Build(true)
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
			"CreationDate",
			string(image.Metadata.CreationDate),
			string(image.creationDate.Unix()),
		},
		{
			"Properties[architecture]",
			image.Metadata.Properties["architecture"],
			"x86_64",
		},
		{
			"Properties[os]",
			image.Metadata.Properties["os"],
			lxdImageDef.Distribution,
		},
		{
			"Properties[release]",
			image.Metadata.Properties["release"],
			lxdImageDef.Release,
		},
		{
			"Properties[description]",
			image.Metadata.Properties["description"],
			fmt.Sprintf("%s %s", strings.Title(lxdImageDef.Distribution),
				lxdImageDef.Release),
		},
		{
			"Properties[name]",
			image.Metadata.Properties["name"],
			fmt.Sprintf("%s-%s-%s-%s", strings.ToLower(lxdImageDef.Distribution),
				lxdImageDef.Release, "x86_64", image.creationDate.Format("20060201_1504")),
		},
		{
			"ExpiryDate",
			fmt.Sprintf("%d", image.Metadata.ExpiryDate),
			"1138763045",
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		if tt.have != tt.expected {
			t.Fatalf("Expected '%s', got '%s'", tt.expected, tt.have)
		}
	}
}
