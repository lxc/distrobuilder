package shared

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	lxd "github.com/lxc/lxd/shared"
)

func TestVerifyFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to retrieve working directory: %v", err)
	}
	testdataDir := filepath.Join(wd, "..", "testdata")

	keys := []string{"0x5DE8949A899C8D99"}
	keyserver := "keys.gnupg.net"

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
		{
			"missing keyserver",
			filepath.Join(testdataDir, "testfile.asc"),
			"",
			keys,
			"",
			false,
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		valid, err := VerifyFile(tt.signedFile, tt.signatureFile, tt.keys,
			tt.keyserver)
		if !tt.shouldFail && !valid {
			t.Fatalf("Failed to verify: %s\n%s", tt.name, err)
		}
		if tt.shouldFail && valid {
			t.Fatalf("Expected to fail: %s", tt.name)
		}
	}
}

func TestCreateGPGKeyring(t *testing.T) {
	gpgDir, err := CreateGPGKeyring("pgp.mit.edu", []string{"0x5DE8949A899C8D99"})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if !lxd.PathExists(gpgDir) {
		t.Fatalf("Failed to create gpg directory: %s", gpgDir)
	}
	os.RemoveAll(gpgDir)

	// This shouldn't fail either.
	gpgDir, err = CreateGPGKeyring("", []string{})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if !lxd.PathExists(gpgDir) {
		t.Fatalf("Failed to create gpg directory: %s", gpgDir)
	}
	os.RemoveAll(gpgDir)
}
