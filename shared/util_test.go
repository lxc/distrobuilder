package shared

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/flosch/pongo2/v4"
	"github.com/stretchr/testify/require"
)

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

func TestParseCompression(t *testing.T) {
	tests := []struct {
		compression         string
		expectedCompression string
		expectLevel         bool
		expectedLevel       int
		shouldFail          bool
	}{
		{
			"gzip", "gzip", false, 0 /* irrelevant */, false,
		},
		{
			"gzip-1", "gzip", true, 1, false,
		},
		{
			"gzip-10", "", false, 0, true,
		},
		{
			"zstd-22", "zstd", true, 22, false,
		},
		{
			"gzip-0", "", false, 0, true,
		},
		{
			"unknown-1", "", false, 0, true,
		},
		{
			"lzo", "lzop", false, 0 /* irrelevant */, false,
		},
		{
			"lzo-9", "lzop", true, 9, false,
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.compression)
		compression, level, err := ParseCompression(tt.compression)

		if tt.shouldFail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tt.expectedCompression, compression)
			if tt.expectLevel {
				require.NotNil(t, level)
				require.Equal(t, tt.expectedLevel, *level)
			}
		}
	}
}

func TestSquashfsParseCompression(t *testing.T) {
	tests := []struct {
		compression         string
		expectedCompression string
		expectLevel         bool
		expectedLevel       int
		shouldFail          bool
	}{
		{
			"gzip", "gzip", false, 0 /* irrelevant */, false,
		},
		{
			"gzip-1", "gzip", true, 1, false,
		},
		{
			"gzip-10", "", false, 0, true,
		},
		{
			"zstd-22", "zstd", true, 22, false,
		},
		{
			"gzip-0", "", false, 0, true,
		},
		{
			"invalid", "", false, 0, true,
		},
		{
			"xz-1", "", false, 0, true,
		},
		{
			"lzop", "lzo", false, 0 /* irrelevant */, false,
		},
		{
			"lzop-9", "lzo", true, 9, false,
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.compression)
		compression, level, err := ParseSquashfsCompression(tt.compression)

		if tt.shouldFail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tt.expectedCompression, compression)
			if tt.expectLevel {
				require.NotNil(t, level)
				require.Equal(t, tt.expectedLevel, *level)
			}
		}
	}
}

func TestCaseInsensitive(t *testing.T) {
	tcs := []struct {
		input string
		want  string
	}{
		{"sources", "[sS][oO][uU][rR][cC][eE][sS]"},
		{"boot.wim", "[bB][oO][oO][tT]\\.[wW][iI][mM]"},
		{"install.wim", "[iI][nN][sS][tT][aA][lL][lL]\\.[wW][iI][mM]"},
		{"sources/boot.wim", "[sS][oO][uU][rR][cC][eE][sS]/[bB][oO][oO][tT]\\.[wW][iI][mM]"},
		{"sources/install.wim", "[sS][oO][uU][rR][cC][eE][sS]/[iI][nN][sS][tT][aA][lL][lL]\\.[wW][iI][mM]"},
	}

	for _, tc := range tcs {
		t.Run(tc.input, func(t *testing.T) {
			pattern := CaseInsensitive(tc.input)
			if pattern != tc.want {
				t.Fatal(pattern, tc.input, tc.want)
			}
		})
	}
}

func TestFindFirstMatch(t *testing.T) {
	tcs := []struct {
		name   string
		actual string
	}{
		{"sources/boot.wim", "sources/boot.wim"},
		{"sources/install.wim", "sources/install.wim"},
		{"sources/install.wim", "SOURCES/INSTALL.wim"},
		{"sources/boot.wim", "sources/BOOT.WIM"},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := filepath.Abs("testing")
			if err != nil {
				t.Fatal(err)
			}

			actual := filepath.Join(dir, tc.actual)
			err = os.MkdirAll(filepath.Dir(actual), 0755)
			if err != nil {
				t.Fatal(err)
			}

			f, err := os.Create(actual)
			if err != nil {
				t.Fatal(err)
			}

			err = f.Close()
			if err != nil {
				t.Fatal(err)
			}

			defer os.RemoveAll(dir)
			want, err := FindFirstMatch(dir, tc.name)
			if err != nil {
				t.Fatal(err)
			}

			if want != actual {
				t.Fatal(want, actual)
			}
		})
	}
}
