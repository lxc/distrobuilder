package shared

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"testing"

	lxd "github.com/lxc/lxd/shared"
	"github.com/stretchr/testify/require"
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
	keyring, err := CreateGPGKeyring("keyserver.ubuntu.com", []string{"0x5DE8949A899C8D99"})
	require.NoError(t, err)

	require.FileExists(t, keyring)
	os.RemoveAll(path.Dir(keyring))

	// This shouldn't fail, but the keyring file should not be created since
	// there are no keys to be exported.
	keyring, err = CreateGPGKeyring("", []string{})
	require.NoError(t, err)

	require.False(t, lxd.PathExists(keyring), "File should not exist")
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
		if tt.shouldFail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tt.expected, ret)
		}
	}
}

func TestSetEnvVariables(t *testing.T) {
	// Initial variables
	os.Setenv("FOO", "bar")

	env := Environment{
		"FOO": EnvVariable{
			Value: "bla",
			Set:   true,
		},
		"BAR": EnvVariable{
			Value: "blub",
			Set:   true,
		},
	}

	// Set new env variables
	oldEnv := SetEnvVariables(env)

	for k, v := range env {
		val, set := os.LookupEnv(k)
		require.True(t, set)
		require.Equal(t, v.Value, val)
	}

	// Reset env variables
	SetEnvVariables(oldEnv)

	val, set := os.LookupEnv("FOO")
	require.True(t, set)
	require.Equal(t, val, "bar")

	val, set = os.LookupEnv("BAR")
	require.False(t, set, "Expected 'BAR' to be unset")
	require.Empty(t, val)
}
