package shared

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/flosch/pongo2.v3"
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
