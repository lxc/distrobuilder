package generators

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	lxd "github.com/lxc/incus/shared"
	"github.com/stretchr/testify/require"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

func TestCloudInitGeneratorRunLXC(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator, err := Load("cloud-init", nil, cacheDir, rootfsDir, shared.DefinitionFile{}, shared.Definition{})
	require.IsType(t, &cloudInit{}, generator)
	require.NoError(t, err)

	// Prepare rootfs
	err = os.MkdirAll(filepath.Join(rootfsDir, "etc", "runlevels"), 0755)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(rootfsDir, "etc", "cloud"), 0755)
	require.NoError(t, err)

	for _, f := range []string{"cloud-init-local", "cloud-config", "cloud-init", "cloud-final"} {
		fullPath := filepath.Join(rootfsDir, "etc", "runlevels", f)
		err = os.Symlink("/dev/null", fullPath)
		require.NoError(t, err)
		require.FileExists(t, fullPath)
	}

	// Disable cloud-init
	err = generator.RunLXC(nil, shared.DefinitionTargetLXC{})
	require.NoError(t, err)

	// Check whether the generator has altered the rootfs
	for _, f := range []string{"cloud-init-local", "cloud-config", "cloud-init", "cloud-final"} {
		fullPath := filepath.Join(rootfsDir, "etc", "runlevels", f)
		require.Falsef(t, lxd.PathExists(fullPath), "File '%s' exists but shouldn't", fullPath)
	}

	for i := 0; i <= 6; i++ {
		dir := filepath.Join(rootfsDir, "etc", "rc.d", fmt.Sprintf("rc%d.d", i))

		for _, f := range []string{"cloud-init-local", "cloud-config", "cloud-init", "cloud-final"} {
			fullPath := filepath.Join(dir, fmt.Sprintf("S99%s", f))
			require.Falsef(t, lxd.PathExists(fullPath), "File '%s' exists but shouldn't", fullPath)
		}
	}

	require.FileExists(t, filepath.Join(rootfsDir, "etc", "cloud", "cloud-init.disabled"))
}

func TestCloudInitGeneratorRunLXD(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	definition := shared.Definition{
		Image: shared.DefinitionImage{
			Distribution: "ubuntu",
			Release:      "artful",
		},
	}

	image := image.NewIncusImage(context.TODO(), cacheDir, "", cacheDir, definition)

	tests := []struct {
		name       string
		expected   string
		shouldFail bool
	}{
		{
			"user-data",
			`{%- if config_get("cloud-init.user-data", properties.default) == properties.default -%}
{{ config_get("user.user-data", properties.default) }}
{%- else -%}
{{- config_get("cloud-init.user-data", properties.default) }}
{%- endif %}
`,
			false,
		},
		{
			"meta-data",
			`instance-id: {{ container.name }}
local-hostname: {{ container.name }}
{{ config_get("user.meta-data", "") }}
`,
			false,
		},
		{
			"vendor-data",
			`{%- if config_get("cloud-init.vendor-data", properties.default) == properties.default -%}
{{ config_get("user.vendor-data", properties.default) }}
{%- else -%}
{{- config_get("cloud-init.vendor-data", properties.default) }}
{%- endif %}
`,
			false,
		},
		{
			"network-config",
			`{%- if config_get("cloud-init.network-config", "") == "" -%}
{%- if config_get("user.network-config", "") == "" -%}
version: 1
config:
  - type: physical
    name: {% if instance.type == "virtual-machine" %}enp5s0{% else %}eth0{% endif %}
    subnets:
      - type: dhcp
        control: auto
{%- else -%}
{{- config_get("user.network-config", "") -}}
{%- endif -%}
{%- else -%}
{{- config_get("cloud-init.network-config", "") -}}
{%- endif %}
`,
			false,
		},
		{
			"foo",
			"Unknown cloud-init configuration: foo",
			true,
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)

		generator, err := Load("cloud-init", nil, cacheDir, rootfsDir, shared.DefinitionFile{
			Generator: "cloud-init",
			Name:      tt.name,
		}, shared.Definition{})
		require.IsType(t, &cloudInit{}, generator)
		require.NoError(t, err)

		err = generator.RunIncus(image, shared.DefinitionTargetIncus{})

		if !tt.shouldFail {
			require.NoError(t, err)
		} else {
			require.Regexp(t, tt.expected, err)
			continue
		}

		validateTestFile(t, filepath.Join(cacheDir, "templates", fmt.Sprintf("cloud-init-%s.tpl", tt.name)), tt.expected)
	}
}
