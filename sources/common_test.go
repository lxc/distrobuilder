package sources

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"testing"

	lxd "github.com/lxc/lxd/shared"
	"github.com/stretchr/testify/require"
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
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		valid, err := c.VerifyFile(tt.signedFile, tt.signatureFile, tt.keys,
			tt.keyserver)
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
	}

	keyring, err := c.CreateGPGKeyring("keyserver.ubuntu.com", []string{"0x5DE8949A899C8D99"})
	require.NoError(t, err)

	require.FileExists(t, keyring)
	os.RemoveAll(path.Dir(keyring))

	// This shouldn't fail, but the keyring file should not be created since
	// there are no keys to be exported.
	keyring, err = c.CreateGPGKeyring("", []string{})
	require.NoError(t, err)

	require.False(t, lxd.PathExists(keyring), "File should not exist")
	os.RemoveAll(path.Dir(keyring))
}
