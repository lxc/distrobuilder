package shared

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/flosch/pongo2/v6"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
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

func findMatchHelper(t *testing.T, filenames ...string) (dir string, actuals []string, rb func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "findmatch*")
	if err != nil {
		t.Fatal(err)
	}

	actuals = make([]string, len(filenames))
	for i, filename := range filenames {
		filename = filepath.Join(dir, filename)
		err = os.MkdirAll(filepath.Dir(filename), 0o755)
		if err != nil {
			t.Fatal(err)
		}

		file, err := os.Create(filename)
		if err != nil {
			t.Fatal(err)
		}

		err = file.Close()
		if err != nil {
			t.Fatal(err)
		}

		actuals[i] = filename
	}

	rb = func() {
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatal(err)
		}
	}

	return dir, actuals, rb
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
			dir, actuals, rb := findMatchHelper(t, tc.actual)
			defer rb()
			actual := actuals[0]
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

func TestFindAllMatches(t *testing.T) {
	tcs := []struct {
		name      string
		filenames []string
	}{
		{"*.inf", []string{"vioinput.inf"}},
		{"*.sys", []string{"viohidkmdf.sys", "vioinput.sys"}},
		{"*.cat", []string{"vioinput.cat"}},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir, actuals, rb := findMatchHelper(t, tc.filenames...)
			defer rb()
			matches, err := FindAllMatches(dir, tc.name)
			if err != nil {
				t.Fatal(err)
			}

			if !slices.Equal(matches, actuals) {
				t.Fatal(matches, actuals)
			}
		})
	}
}

func TestRunScript(t *testing.T) {
	// Behaves as vm.memfd_noexec=2, where memfd_create refuses executable memfds.
	memfdDenied := func(string, int) (int, error) {
		return -1, unix.EACCES
	}

	// Behaves as a kernel older than 6.3, which doesn't know MFD_EXEC and
	// always creates executable memfds.
	memfdWithoutExecFlag := func(name string, flags int) (int, error) {
		if flags != 0 {
			return -1, unix.EINVAL
		}

		return unix.MemfdCreate(name, unix.MFD_EXEC)
	}

	tcs := []struct {
		name          string
		script        string
		memfdCreate   memfdCreateFunc
		expectedFlags []int
		fromFile      bool
		exitCode      int
		noOutput      bool
	}{
		{
			name:        "script succeeds",
			script:      "#!/bin/sh\necho \"$0\" > \"%s\"\n",
			memfdCreate: unix.MemfdCreate,
		},
		{
			name:        "script fails",
			script:      "#!/bin/sh\necho \"$0\" > \"%s\"\nexit 3\n",
			memfdCreate: unix.MemfdCreate,
			exitCode:    3,
		},
		{
			name:          "memfd without MFD_EXEC support",
			script:        "#!/bin/sh\necho \"$0\" > \"%s\"\n",
			memfdCreate:   memfdWithoutExecFlag,
			expectedFlags: []int{unix.MFD_EXEC, 0},
		},
		{
			name:          "memfd denied, script succeeds",
			script:        "#!/bin/sh\necho \"$0\" > \"%s\"\n",
			memfdCreate:   memfdDenied,
			expectedFlags: []int{unix.MFD_EXEC},
			fromFile:      true,
		},
		{
			name:          "memfd denied, script fails",
			script:        "#!/bin/sh\necho \"$0\" > \"%s\"\nexit 3\n",
			memfdCreate:   memfdDenied,
			expectedFlags: []int{unix.MFD_EXEC},
			fromFile:      true,
			exitCode:      3,
		},
		{
			// The kernel passes the shebang's options to the interpreter, so
			// the script stops at "false" and never writes its output.
			name:          "memfd denied, shebang options are honored",
			script:        "#!/bin/sh -e\nfalse\necho \"$0\" > \"%s\"\n",
			memfdCreate:   memfdDenied,
			expectedFlags: []int{unix.MFD_EXEC},
			fromFile:      true,
			exitCode:      1,
			noOutput:      true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			scriptDir := t.TempDir()
			t.Setenv("TMPDIR", scriptDir)

			output := filepath.Join(t.TempDir(), "output")

			var flags []int

			memfdCreate := func(name string, flag int) (int, error) {
				flags = append(flags, flag)

				return tc.memfdCreate(name, flag)
			}

			err := runScript(context.Background(), fmt.Sprintf(tc.script, output), memfdCreate)

			if tc.exitCode == 0 {
				require.NoError(t, err)
			} else {
				var exitErr *exec.ExitError

				require.ErrorAs(t, err, &exitErr)
				require.Equal(t, tc.exitCode, exitErr.ExitCode())
			}

			if tc.expectedFlags != nil {
				require.Equal(t, tc.expectedFlags, flags)
			}

			if tc.noOutput {
				require.NoFileExists(t, output)
			} else {
				// The script writes the path it was started from.
				startedFrom, err := os.ReadFile(output)
				require.NoError(t, err)

				if tc.fromFile {
					require.Truef(t, strings.HasPrefix(string(startedFrom), filepath.Join(scriptDir, ".distrobuilder-script-")), "script started from %q", startedFrom)
				}
			}

			leftovers, err := os.ReadDir(scriptDir)
			require.NoError(t, err)
			require.Empty(t, leftovers)
		})
	}
}
