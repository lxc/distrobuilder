package shared

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Errorf("Failed to retrieve working directory: %s", err)
	}
	testdataDir := filepath.Join(wd, "..", "testdata")

	keys := []string{"0x5DE8949A899C8D99"}

	tests := []struct {
		name          string
		signedFile    string
		signatureFile string
		keys          []string
		shouldFail    bool
	}{
		{
			"testfile with detached signature",
			filepath.Join(testdataDir, "testfile"),
			filepath.Join(testdataDir, "testfile.sig"),
			keys,
			false,
		},
		{
			"testfile with cleartext signature",
			filepath.Join(testdataDir, "testfile.asc"),
			"",
			keys,
			false,
		},
		{
			"testfile with invalid cleartext signature",
			filepath.Join(testdataDir, "testfile-invalid.asc"),
			"",
			keys,
			true,
		},
		{
			"testfile with normal signature",
			filepath.Join(testdataDir, "testfile.gpg"),
			"",
			keys,
			false,
		},
		{
			"no keys",
			filepath.Join(testdataDir, "testfile"),
			filepath.Join(testdataDir, "testfile.sig"),
			[]string{},
			true,
		},
		{
			"invalid key",
			filepath.Join(testdataDir, "testfile.asc"),
			"",
			[]string{"0x46181433FBB75451"},
			true,
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		valid, err := VerifyFile(tt.signedFile, tt.signatureFile, tt.keys)
		if !tt.shouldFail && !valid {
			t.Fatalf("Failed to verify: %s\n%s", tt.name, err)
		}
		if tt.shouldFail && valid {
			t.Fatalf("Expected to fail: %s", tt.name)
		}
	}
}
