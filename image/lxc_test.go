package image

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"syscall"
	"testing"

	"github.com/lxc/distrobuilder/shared"
)

var lxcImageDef = shared.DefinitionImage{
	Description:  "{{ image. Distribution|capfirst }} {{ image.Release }}",
	Distribution: "ubuntu",
	Release:      "17.10",
	Architecture: "amd64",
	Expiry:       "30d",
	Name:         "{{ image.Distribution|lower }}-{{ image.Release }}-{{ image.Architecture }}-{{ creation_date }}",
}

var lxcTarget = shared.DefinitionTargetLXC{
	CreateMessage: "Welcome to {{ image.Distribution|capfirst}} {{ image.Release }}",
	Config: []shared.DefinitionTargetLXCConfig{
		{
			Type:    "all",
			Before:  5,
			Content: "all_before_5",
		},
		{
			Type:    "user",
			Before:  5,
			Content: "user_before_5",
		},
		{
			Type:    "all",
			After:   4,
			Content: "all_after_4",
		},
		{
			Type:    "user",
			After:   4,
			Content: "user_after_4",
		},
		{
			Type:    "all",
			Content: "all",
		},
		{
			Type:    "system",
			Before:  2,
			Content: "system_before_2",
		},
		{
			Type:    "system",
			Before:  2,
			After:   4,
			Content: "system_before_2_after_4",
		},
		{
			Type:    "user",
			Before:  3,
			After:   3,
			Content: "user_before_3_after_3",
		},
		{
			Type:    "system",
			Before:  4,
			After:   2,
			Content: "system_before_4_after_2",
		},
	},
}

func lxcCacheDir() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "distrobuilder-test")
}

func setupLXC() *LXCImage {
	return NewLXCImage(lxcCacheDir(), "", lxcCacheDir(), lxcImageDef, lxcTarget)
}

func teardownLXC() {
	os.RemoveAll(lxcCacheDir())
}

func TestNewLXCImage(t *testing.T) {
	image := NewLXCImage(lxcCacheDir(), "", lxcCacheDir(), lxcImageDef, lxcTarget)
	defer teardownLXC()

	if image.cacheDir != lxcCacheDir() {
		t.Fatalf("Expected image.cacheDir to be '%s', got '%s'", lxcCacheDir(),
			image.cacheDir)
	}

	if !reflect.DeepEqual(image.definition, lxcImageDef) {
		t.Fatalf("lxcImageDef and image.definition are not equal")
	}

	if !reflect.DeepEqual(image.target, lxcTarget) {
		t.Fatalf("lxcTarget and image.target are not equal")
	}
}

func TestLXCAddTemplate(t *testing.T) {
	image := setupLXC()
	defer teardownLXC()

	// Make sure templates file is empty.
	info, err := os.Stat(filepath.Join(lxcCacheDir(), "metadata", "templates"))
	if err == nil && info.Size() > 0 {
		t.Fatalf("Expected file size to be 0, got %d", info.Size())
	}

	// Add first template entry.
	image.AddTemplate("/path/file1")
	file, err := os.Open(filepath.Join(lxcCacheDir(), "metadata", "templates"))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Copy file content to buffer.
	var buffer bytes.Buffer
	io.Copy(&buffer, file)
	file.Close()

	if buffer.String() != "/path/file1\n" {
		t.Fatalf("Expected templates content to be '%s', got '%s'",
			"/path/file", buffer.String())
	}

	// Add second template entry.
	image.AddTemplate("/path/file2")
	file, err = os.Open(filepath.Join(lxcCacheDir(), "metadata", "templates"))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Copy file content to buffer.
	buffer.Reset()
	io.Copy(&buffer, file)
	file.Close()

	if buffer.String() != "/path/file1\n/path/file2\n" {
		t.Fatalf("Expected templates content to be '%s', got '%s'",
			"/path/file1\n/path/file2", buffer.String())
	}
}

func TestLXCBuild(t *testing.T) {
	image := setupLXC()
	defer teardownLXC()

	err := os.MkdirAll(filepath.Join(lxcCacheDir(), "rootfs"), 0755)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	err = image.Build()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer func() {
		os.Remove("meta.tar.xz")
		os.Remove("rootfs.tar.xz")
	}()
}

func TestLXCCreateMetadataBasic(t *testing.T) {
	defaultImage := setupLXC()
	defer teardownLXC()

	tests := []struct {
		name          string
		shouldFail    bool
		expectedError string
		prepareImage  func(LXCImage) *LXCImage
	}{
		{
			"valid metadata",
			false,
			"",
			func(l LXCImage) *LXCImage { return &l },
		},
		{
			"invalid config template",
			true,
			"Error writing 'config': .+",
			func(l LXCImage) *LXCImage {
				l.target.Config = []shared.DefinitionTargetLXCConfig{
					{
						Type:    "all",
						After:   4,
						Content: "{{ invalid }",
					},
				}
				return &l
			},
		},
		{
			"invalid create-message template",
			true,
			"Error writing 'create-message': .+",
			func(l LXCImage) *LXCImage {
				l.target.CreateMessage = "{{ invalid }"
				return &l
			},
		},
		{
			"existing dev directory",
			false,
			"",
			func(l LXCImage) *LXCImage {
				// Create /dev and device file.
				os.MkdirAll(filepath.Join(lxcCacheDir(), "rootfs", "dev"), 0755)
				syscall.Mknod(filepath.Join(lxcCacheDir(), "rootfs", "dev", "null"),
					syscall.S_IFCHR, 0)
				return &l
			},
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		image := tt.prepareImage(*defaultImage)
		err := image.createMetadata()
		if tt.shouldFail {
			if err == nil {
				t.Fatal("Expected to fail, but didn't")
			}

			match, _ := regexp.MatchString(tt.expectedError, err.Error())
			if !match {
				t.Fatalf("Expected to fail with '%s', got '%s'", tt.expectedError,
					err.Error())
			}
		}
		if !tt.shouldFail && err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
	}
}

func TestLXCCreateMetadataConfig(t *testing.T) {
	image := setupLXC()
	defer teardownLXC()

	tests := []struct {
		configFile string
		expected   string
	}{
		{
			"config",
			"all_after_4\nall\nsystem_before_2_after_4\n",
		},
		{
			"config.1",
			"all_before_5\nall\nsystem_before_2\nsystem_before_2_after_4\n",
		},
		{
			"config.2",
			"all_before_5\nall\n",
		},
		{
			"config.3",
			"all_before_5\nall\nsystem_before_4_after_2\n",
		},
		{
			"config.4",
			"all_before_5\nall\n",
		},
		{
			"config.user",
			"all_after_4\nuser_after_4\nall\nuser_before_3_after_3\n",
		},
		{
			"config.user.1",
			"all_before_5\nuser_before_5\nall\nuser_before_3_after_3\n",
		},
		{
			"config.user.2",
			"all_before_5\nuser_before_5\nall\nuser_before_3_after_3\n",
		},
		{
			"config.user.3",
			"all_before_5\nuser_before_5\nall\n",
		},
		{
			"config.user.4",
			"all_before_5\nuser_before_5\nall\nuser_before_3_after_3\n",
		},
	}

	err := image.createMetadata()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	for _, tt := range tests {
		log.Printf("Checking '%s'", tt.configFile)
		file, err := os.Open(filepath.Join(lxcCacheDir(), "metadata", tt.configFile))
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		var buffer bytes.Buffer
		_, err = io.Copy(&buffer, file)
		file.Close()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if buffer.String() != tt.expected {
			t.Fatalf("Expected '%s', got '%s'", tt.expected, buffer.String())
		}
	}
}

func TestLXCPackMetadata(t *testing.T) {
	image := setupLXC()
	defer func() {
		teardownLXC()
		os.Remove("meta.tar.xz")
	}()

	err := image.createMetadata()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	err = image.packMetadata()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Include templates directory.
	image.AddTemplate("/path/file")
	err = image.packMetadata()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Provoke error by removing the metadata directory
	os.RemoveAll(filepath.Join(lxcCacheDir(), "metadata"))
	err = image.packMetadata()
	if err == nil {
		t.Fatal("Expected failure")
	}

}

func TestLXCWriteMetadata(t *testing.T) {
	image := setupLXC()
	defer teardownLXC()

	// Should fail due to invalid path
	err := image.writeMetadata("/path/file", "", false)
	if err == nil {
		t.Fatal("Expected failure")
	}

	// Should succeed
	err = image.writeMetadata("test", "metadata", false)
	if err != nil {
		t.Fatalf("Unexpected failure: %s", err)
	}
	os.Remove("test")
}
