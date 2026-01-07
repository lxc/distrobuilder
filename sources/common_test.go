package sources

import (
	"context"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"testing"

	incus "github.com/lxc/incus/v6/shared/util"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/lxc/distrobuilder/shared"
)

func TestVerifyFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to retrieve working directory: %v", err)
	}

	testdataDir := filepath.Join(wd, "..", "testdata")

	keys := []string{"0x5DE8949A899C8D99"}
	keyserver := "keyserver.ubuntu.com"

	tests := []struct {
		name          string
		signedFile    string
		signatureFile string
		keys          []string
		keyserver     string
		shouldFail    bool
	}{
		{
			"testfile with detached signature",
			filepath.Join(testdataDir, "testfile"),
			filepath.Join(testdataDir, "testfile.sig"),
			keys,
			keyserver,
			false,
		},
		{
			"testfile with cleartext signature",
			filepath.Join(testdataDir, "testfile.asc"),
			"",
			keys,
			keyserver,
			false,
		},
		{
			"testfile with invalid cleartext signature",
			filepath.Join(testdataDir, "testfile-invalid.asc"),
			"",
			keys,
			keyserver,
			true,
		},
		{
			"testfile with normal signature",
			filepath.Join(testdataDir, "testfile.gpg"),
			"",
			keys,
			keyserver,
			false,
		},
		{
			"no keys",
			filepath.Join(testdataDir, "testfile"),
			filepath.Join(testdataDir, "testfile.sig"),
			[]string{},
			keyserver,
			true,
		},
		{
			"invalid key",
			filepath.Join(testdataDir, "testfile.asc"),
			"",
			[]string{"0x46181433FBB75451"},
			keyserver,
			true,
		},
	}

	c := common{
		sourcesDir: os.TempDir(),
		definition: shared.Definition{
			Source: shared.DefinitionSource{},
		},
		ctx: context.TODO(),
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)

		c.definition = shared.Definition{
			Source: shared.DefinitionSource{
				Keyserver: tt.keyserver,
				Keys:      tt.keys,
			},
		}

		valid, err := c.VerifyFile(tt.signedFile, tt.signatureFile)
		if tt.shouldFail {
			require.Error(t, err)
			require.False(t, valid)
		} else {
			require.NoError(t, err)
			require.True(t, valid)
		}
	}
}

func TestCreateGPGKeyring(t *testing.T) {
	c := common{
		sourcesDir: os.TempDir(),
		definition: shared.Definition{
			Source: shared.DefinitionSource{
				Keyserver: "keyserver.ubuntu.com",
				Keys:      []string{"0x5DE8949A899C8D99"},
			},
		},
		ctx: context.TODO(),
	}

	keyring, err := c.CreateGPGKeyring()
	require.NoError(t, err)

	require.FileExists(t, keyring)
	os.RemoveAll(path.Dir(keyring))

	c.definition = shared.Definition{}

	// This shouldn't fail, but the keyring file should not be created since
	// there are no keys to be exported.
	keyring, err = c.CreateGPGKeyring()
	require.NoError(t, err)

	require.False(t, incus.PathExists(keyring), "File should not exist")
	os.RemoveAll(path.Dir(keyring))
}

func TestValidateGPGRequirements(t *testing.T) {
	tests := []struct {
		name             string
		url              string
		keys             []string
		skipVerification bool
		expectError      bool
		expectSkipVerify bool
	}{
		{
			name:             "keys provided with https",
			url:              "https://example.com/file",
			keys:             []string{"0x12345678"},
			skipVerification: false,
			expectError:      false,
			expectSkipVerify: false, // Keys provided, always verify
		},
		{
			name:             "keys provided with http",
			url:              "http://example.com/file",
			keys:             []string{"0x12345678"},
			skipVerification: false,
			expectError:      false,
			expectSkipVerify: false, // Keys provided, always verify
		},
		{
			name:             "http without keys",
			url:              "http://example.com/file",
			keys:             []string{},
			skipVerification: false,
			expectError:      true, // HTTP requires GPG keys
			expectSkipVerify: false,
		},
		{
			name:             "https without keys and skip false",
			url:              "https://example.com/file",
			keys:             []string{},
			skipVerification: false,
			expectError:      false,
			expectSkipVerify: true, // Should be set to true with warning
		},
		{
			name:             "https without keys and skip true",
			url:              "https://example.com/file",
			keys:             []string{},
			skipVerification: true,
			expectError:      false,
			expectSkipVerify: true, // Remains true
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			logger.SetOutput(os.Stdout)

			c := common{
				logger: logger,
				definition: shared.Definition{
					Source: shared.DefinitionSource{
						Keys:             tt.keys,
						SkipVerification: tt.skipVerification,
					},
				},
				ctx: context.TODO(),
			}

			u, err := url.Parse(tt.url)
			require.NoError(t, err)

			skip, err := c.validateGPGRequirements(u)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectSkipVerify, skip)
			}
		})
	}
}
