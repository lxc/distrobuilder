package generators

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
	"github.com/stretchr/testify/require"
)

func TestCloudInitGeneratorRunLXD(t *testing.T) {

	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator := Get("cloud-init")
	require.Equal(t, CloudInitGenerator{}, generator)

	definition := shared.Definition{
		Image: shared.DefinitionImage{
			Distribution: "ubuntu",
			Release:      "artful",
		},
	}

	image := image.NewLXDImage(cacheDir, "", cacheDir, definition)

	tests := []struct {
		name       string
		expected   string
		shouldFail bool
	}{
		{
			"user-data",
			`{{ config_get("user.user-data", properties.default) }}
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
			`{{ config_get("user.vendor-data", properties.default) }}
`,
			false,
		},
		{
			"network-config",
			`{% if config_get("user.network-config", "") == "" %}version: 1
config:
  - type: physical
    name: eth0
    subnets:
      - type: {% if config_get("user.network_mode", "") == "link-local" %}manual{% else %}dhcp{% endif %}
        control: auto{% else %}{{ config_get("user.network-config", "") }}{% endif %}
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

		err := generator.RunLXD(cacheDir, rootfsDir, image, shared.DefinitionFile{
			Generator: "cloud-init",
			Name:      tt.name,
		})

		if !tt.shouldFail {
			require.NoError(t, err)
		} else {
			require.Regexp(t, tt.expected, err)
			continue
		}

		validateTestFile(t, filepath.Join(cacheDir, "templates", fmt.Sprintf("cloud-init-%s.tpl", tt.name)), tt.expected)
	}

}
