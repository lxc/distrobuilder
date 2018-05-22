package shared

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"testing"

	lxd "github.com/lxc/lxd/shared"
	"gopkg.in/flosch/pongo2.v3"
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
	keyring, err := CreateGPGKeyring("keyserver.ubuntu.com", []string{"0x5DE8949A899C8D99"})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if !lxd.PathExists(keyring) {
		t.Fatalf("Failed to create GPG keyring '%s'", keyring)
	}
	os.RemoveAll(path.Dir(keyring))

	// This shouldn't fail, but the keyring file should not be created since
	// there are no keys to be exported.
	keyring, err = CreateGPGKeyring("", []string{})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if lxd.PathExists(keyring) {
		t.Fatalf("GPG keyring '%s' should not exist", keyring)
	}
	os.RemoveAll(path.Dir(keyring))
}

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name       string
		iface      interface{}
		template   string
		expected   string
		shouldFail bool
	}{
		{
			"valid template with yaml tags",
			Definition{
				Image: DefinitionImage{
					Distribution: "Ubuntu",
					Release:      "Bionic",
				},
			},
			"{{ image.distribution }} {{ image.release }}",
			"Ubuntu Bionic",
			false,
		},
		{
			"valid template without yaml tags",
			pongo2.Context{
				"foo": "bar",
			},
			"{{ foo }}",
			"bar",
			false,
		},
		{
			"variable not in context",
			pongo2.Context{},
			"{{ foo }}",
			"",
			false,
		},
		{
			"invalid template",
			pongo2.Context{
				"foo": nil,
			},
			"{{ foo }",
			"",
			true,
		},
		{
			"invalid context",
			pongo2.Context{
				"foo.bar": nil,
			},
			"{{ foo.bar }}",
			"",
			true,
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		ret, err := RenderTemplate(tt.template, tt.iface)
		if tt.shouldFail && err == nil {
			t.Fatal("test should have failed")
		}
		if ret != tt.expected {
			t.Fatalf("expected '%s', got '%s'", tt.expected, ret)
		}
	}
}

func TestSetEnvVariables(t *testing.T) {
	// Initial variables
	os.Setenv("FOO", "bar")

	env := []EnvVariable{
		{"FOO", "bla", true},
		{"BAR", "blub", true},
	}

	// Set new env variables
	oldEnv := SetEnvVariables(env)

	for _, e := range env {
		v, set := os.LookupEnv(e.Key)
		if !set || e.Value != v {
			t.Fatalf("Expected %s to be '%s', got '%s'", e.Key, e.Value, v)
		}
	}

	// Reset env variables
	SetEnvVariables(oldEnv)

	val, set := os.LookupEnv("FOO")
	if !set || val != "bar" {
		t.Fatalf("Expected %s to be '%s', got '%s'", "FOO", "bar", val)
	}

	val, set = os.LookupEnv("BAR")
	if set {
		t.Fatalf("Expected %s to be unset", "BAR")
	}
}
