package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	incus "github.com/lxc/incus/shared"
	"github.com/lxc/incus/shared/api"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

type cloudInit struct {
	common
}

// RunLXC disables cloud-init.
func (g *cloudInit) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {
	// With OpenRC:
	// Remove all symlinks to /etc/init.d/cloud-{init-local,config,init,final} in /etc/runlevels/*
	fullPath := filepath.Join(g.sourceDir, "etc", "runlevels")

	if incus.PathExists(fullPath) {
		err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			if incus.StringInSlice(info.Name(), []string{"cloud-init-local", "cloud-config", "cloud-init", "cloud-final"}) {
				err := os.Remove(path)
				if err != nil {
					return fmt.Errorf("Failed to remove file %q: %w", path, err)
				}
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("Failed to walk file tree %q: %w", fullPath, err)
		}
	}

	// With systemd:
	path := filepath.Join(g.sourceDir, "/etc/cloud")

	if !incus.PathExists(path) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", path, err)
		}
	}

	// Create file /etc/cloud/cloud-init.disabled
	path = filepath.Join(g.sourceDir, "/etc/cloud/cloud-init.disabled")

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("Failed to create file %q: %w", path, err)
	}

	defer f.Close()

	return nil
}

// RunIncus creates cloud-init template files.
func (g *cloudInit) RunIncus(img *image.IncusImage, target shared.DefinitionTargetIncus) error {
	templateDir := filepath.Join(g.cacheDir, "templates")

	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", templateDir, err)
	}

	var content string
	properties := make(map[string]string)

	switch g.defFile.Name {
	case "user-data":
		content = `{%- if config_get("cloud-init.user-data", properties.default) == properties.default -%}
{{ config_get("user.user-data", properties.default) }}
{%- else -%}
{{- config_get("cloud-init.user-data", properties.default) }}
{%- endif %}
`
		properties["default"] = `#cloud-config
{}`
	case "meta-data":
		content = `instance-id: {{ container.name }}
local-hostname: {{ container.name }}
{{ config_get("user.meta-data", "") }}
`
	case "vendor-data":
		content = `{%- if config_get("cloud-init.vendor-data", properties.default) == properties.default -%}
{{ config_get("user.vendor-data", properties.default) }}
{%- else -%}
{{- config_get("cloud-init.vendor-data", properties.default) }}
{%- endif %}
`
		properties["default"] = `#cloud-config
{}`
	case "network-config":
		defaultValue := `version: 1
config:
  - type: physical
    name: {% if instance.type == "virtual-machine" %}enp5s0{% else %}eth0{% endif %}
    subnets:
      - type: dhcp
        control: auto`

		// Use the provided content as the new default value
		if g.defFile.Content != "" {
			defaultValue = g.defFile.Content
		}

		content = fmt.Sprintf(`{%%- if config_get("cloud-init.network-config", "") == "" -%%}
{%%- if config_get("user.network-config", "") == "" -%%}
%s
{%%- else -%%}
{{- config_get("user.network-config", "") -}}
{%%- endif -%%}
{%%- else -%%}
{{- config_get("cloud-init.network-config", "") -}}
{%%- endif %%}
`, defaultValue)
	default:
		return fmt.Errorf("Unknown cloud-init configuration: %s", g.defFile.Name)
	}

	template := fmt.Sprintf("cloud-init-%s.tpl", g.defFile.Name)
	path := filepath.Join(templateDir, template)

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("Failed to create file %q: %w", path, err)
	}

	defer file.Close()

	// Use the provided content as the new default value
	if g.defFile.Name != "network-config" && g.defFile.Content != "" {
		properties["default"] = g.defFile.Content
	}

	// Append final new line if missing
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("Failed to write to content to %s template: %w", g.defFile.Name, err)
	}

	if len(g.defFile.Template.Properties) > 0 {
		properties = g.defFile.Template.Properties
	}

	targetPath := filepath.Join("/var/lib/cloud/seed/nocloud-net", g.defFile.Name)

	if g.defFile.Path != "" {
		targetPath = g.defFile.Path
	}

	// Add to LXD templates
	img.Metadata.Templates[targetPath] = &api.ImageMetadataTemplate{
		Template:   template,
		Properties: properties,
		When:       []string{"create", "copy"},
	}

	return nil
}

// Run does nothing.
func (g *cloudInit) Run() error {
	return nil
}
