package sources

import (
	"context"
	"log"
	"os"
	"path"
	"path/filepath"
	"testing"

	lxd "github.com/lxc/lxd/shared"
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

	require.False(t, lxd.PathExists(keyring), "File should not exist")
	os.RemoveAll(path.Dir(keyring))
}
