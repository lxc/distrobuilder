package image

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/distrobuilder/shared"
)

var lxcDef = shared.Definition{
	Image: shared.DefinitionImage{
		Distribution: "ubuntu",
		Release:      "17.10",
		Architecture: "amd64",
		Expiry:       "30d",
	},
	Targets: shared.DefinitionTarget{
		LXC: shared.DefinitionTargetLXC{
			CreateMessage: "Welcome to {{ image.distribution|capfirst}} {{ image.release }}",
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
		},
	},
}

func lxcCacheDir() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "distrobuilder-test")
}

func setupLXC() *LXCImage {
	return NewLXCImage(lxcCacheDir(), "", lxcCacheDir(), lxcDef)
}

func teardownLXC() {
	os.RemoveAll(lxcCacheDir())
}

func TestNewLXCImage(t *testing.T) {
	image := NewLXCImage(lxcCacheDir(), "", lxcCacheDir(), lxcDef)
	defer teardownLXC()

	require.Equal(t, lxcCacheDir(), image.cacheDir)
	require.Equal(t, lxcDef, image.definition)
}

func TestLXCAddTemplate(t *testing.T) {
	image := setupLXC()
	defer teardownLXC()

	// Make sure templates file is empty.
	_, err := os.Stat(filepath.Join(lxcCacheDir(), "metadata", "templates"))
	require.EqualError(t, err, fmt.Sprintf("stat %s: no such file or directory",
		filepath.Join(lxcCacheDir(), "metadata", "templates")))

	// Add first template entry.
	image.AddTemplate("/path/file1")
	file, err := os.Open(filepath.Join(lxcCacheDir(), "metadata", "templates"))
	require.NoError(t, err)

	// Copy file content to buffer.
	var buffer bytes.Buffer
	io.Copy(&buffer, file)
	file.Close()

	require.Equal(t, "/path/file1\n", buffer.String())

	// Add second template entry.
	image.AddTemplate("/path/file2")
	file, err = os.Open(filepath.Join(lxcCacheDir(), "metadata", "templates"))
	require.NoError(t, err)

	// Copy file content to buffer.
	buffer.Reset()
	io.Copy(&buffer, file)
	file.Close()

	require.Equal(t, "/path/file1\n/path/file2\n", buffer.String())
}

func TestLXCBuild(t *testing.T) {
	image := setupLXC()
	defer teardownLXC()

	err := os.MkdirAll(filepath.Join(lxcCacheDir(), "rootfs"), 0755)
	require.NoError(t, err)

	err = image.Build()
	require.NoError(t, err)
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
				l.definition.Targets.LXC.Config = []shared.DefinitionTargetLXCConfig{
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
				l.definition.Targets.LXC.CreateMessage = "{{ invalid }"
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
			require.Regexp(t, tt.expectedError, err)
		} else {
			require.NoError(t, err)
		}
	}

	// Verify create-message template
	f, err := os.Open(filepath.Join(lxcCacheDir(), "metadata", "create-message"))
	require.NoError(t, err)
	defer f.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, f)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("Welcome to %s %s\n",
		strings.Title(lxcDef.Image.Distribution), lxcDef.Image.Release),
		buf.String())
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
			"config-user",
			"all_after_4\nuser_after_4\nall\nuser_before_3_after_3\n",
		},
		{
			"config-user.1",
			"all_before_5\nuser_before_5\nall\nuser_before_3_after_3\n",
		},
		{
			"config-user.2",
			"all_before_5\nuser_before_5\nall\nuser_before_3_after_3\n",
		},
		{
			"config-user.3",
			"all_before_5\nuser_before_5\nall\n",
		},
		{
			"config-user.4",
			"all_before_5\nuser_before_5\nall\nuser_before_3_after_3\n",
		},
	}

	err := image.createMetadata()
	require.NoError(t, err)

	for _, tt := range tests {
		log.Printf("Checking '%s'", tt.configFile)
		file, err := os.Open(filepath.Join(lxcCacheDir(), "metadata", tt.configFile))
		require.NoError(t, err)

		var buffer bytes.Buffer
		_, err = io.Copy(&buffer, file)
		file.Close()
		require.NoError(t, err)
		require.Equal(t, tt.expected, buffer.String())
	}
}

func TestLXCPackMetadata(t *testing.T) {
	image := setupLXC()
	defer func() {
		teardownLXC()
		os.Remove("meta.tar.xz")
	}()

	err := image.createMetadata()
	require.NoError(t, err)

	err = image.packMetadata()
	require.NoError(t, err)

	// Include templates directory.
	image.AddTemplate("/path/file")
	err = image.packMetadata()
	require.NoError(t, err)

	// Provoke error by removing the metadata directory
	os.RemoveAll(filepath.Join(lxcCacheDir(), "metadata"))
	err = image.packMetadata()
	require.Error(t, err)

}

func TestLXCWriteMetadata(t *testing.T) {
	image := setupLXC()
	defer teardownLXC()

	// Should fail due to invalid path
	err := image.writeMetadata("/path/file", "", false)
	require.Error(t, err)

	// Should succeed
	err = image.writeMetadata("test", "metadata", false)
	require.NoError(t, err)
	os.Remove("test")
}
