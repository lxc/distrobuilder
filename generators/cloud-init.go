package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/flosch/pongo2"
	lxd "github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"
	"github.com/pkg/errors"

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

	if lxd.PathExists(fullPath) {
		err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			if lxd.StringInSlice(info.Name(), []string{"cloud-init-local", "cloud-config", "cloud-init", "cloud-final"}) {
				err := os.Remove(path)
				if err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			return err
		}
	}

	// With upstart:
	// Remove all symlinks to /etc/rc.d/init.d/cloud-{init-local,config,init,final} in /etc/rc.d/rc<runlevel>.d/*
	re := regexp.MustCompile(`^[KS]\d+cloud-(?:config|final|init|init-local)$`)

	for i := 0; i <= 6; i++ {
		fullPath := filepath.Join(g.sourceDir, fmt.Sprintf("/etc/rc.d/rc%d.d", i))

		if !lxd.PathExists(fullPath) {
			continue
		}

		filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			if re.MatchString(info.Name()) {
				err := os.Remove(path)
				if err != nil {
					return err
				}
			}

			return nil
		})
	}

	// With systemd:
	if !lxd.PathExists(filepath.Join(g.sourceDir, "/etc/cloud")) {
		err := os.MkdirAll(filepath.Join(g.sourceDir, "/etc/cloud"), 0755)
		if err != nil {
			return err
		}
	}

	// Create file /etc/cloud/cloud-init.disabled
	f, err := os.Create(filepath.Join(g.sourceDir, "/etc/cloud/cloud-init.disabled"))
	if err != nil {
		return err
	}
	defer f.Close()

	return nil
}

// RunLXD creates cloud-init template files.
func (g *cloudInit) RunLXD(img *image.LXDImage, target shared.DefinitionTargetLXD) error {
	templateDir := filepath.Join(g.cacheDir, "templates")

	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		return err
	}

	var content string
	properties := make(map[string]string)

	switch g.defFile.Name {
	case "user-data":
		content = `{{ config_get("user.user-data", properties.default) }}
`
		properties["default"] = `#cloud-config
{}`
	case "meta-data":
		content = `instance-id: {{ container.name }}
local-hostname: {{ container.name }}
{{ config_get("user.meta-data", "") }}
`
	case "vendor-data":
		content = `{{ config_get("user.vendor-data", properties.default) }}
`
		properties["default"] = `#cloud-config
{}`
	case "network-config":
		content = `{% if config_get("user.network-config", "") == "" %}version: 1
config:
  - type: physical
    name: {% if instance.type == "virtual-machine" %}enp5s0{% else %}eth0{% endif %}
    subnets:
      - type: {% if config_get("user.network_mode", "") == "link-local" %}manual{% else %}dhcp{% endif %}
        control: auto{% else %}{{ config_get("user.network-config", "") }}{% endif %}
`
	default:
		return fmt.Errorf("Unknown cloud-init configuration: %s", g.defFile.Name)
	}

	template := fmt.Sprintf("cloud-init-%s.tpl", g.defFile.Name)

	file, err := os.Create(filepath.Join(templateDir, template))
	if err != nil {
		return err
	}

	defer file.Close()

	if g.defFile.Content != "" {
		content = g.defFile.Content
	}

	// Append final new line if missing
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if g.defFile.Pongo {
		tpl, err := pongo2.FromString(content)
		if err != nil {
			return err
		}

		content, err = tpl.Execute(pongo2.Context{"lxd": target})
		if err != nil {
			return err
		}
	}

	_, err = file.WriteString(content)
	if err != nil {
		return errors.Wrapf(err, "Failed to write to content to %s template", g.defFile.Name)
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

	return err
}

// Run does nothing.
func (g *cloudInit) Run() error {
	return nil
}
