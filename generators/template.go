package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/canonical/lxd/shared/api"
	"github.com/flosch/pongo2"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

type template struct {
	common
}

// RunLXC dumps content to a file.
func (g *template) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {
	// no template support for LXC, ignoring generator
	return nil
}

// RunLXD dumps content to a file.
func (g *template) RunLXD(img *image.LXDImage, target shared.DefinitionTargetLXD) error {
	templateDir := filepath.Join(g.cacheDir, "templates")

	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", templateDir, err)
	}

	template := fmt.Sprintf("%s.tpl", g.defFile.Name)

	file, err := os.Create(filepath.Join(templateDir, template))
	if err != nil {
		return fmt.Errorf("Failed to create file %q: %w", filepath.Join(templateDir, template), err)
	}

	defer file.Close()

	content := g.defFile.Content

	// Append final new line if missing
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if g.defFile.Pongo {
		tpl, err := pongo2.FromString(content)
		if err != nil {
			return fmt.Errorf("Failed to parse template: %w", err)
		}

		content, err = tpl.Execute(pongo2.Context{"lxd": target})
		if err != nil {
			return fmt.Errorf("Failed to execute template: %w", err)
		}
	}

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("Failed to write to content to %s template: %w", g.defFile.Name, err)
	}

	// Add to LXD templates
	img.Metadata.Templates[g.defFile.Path] = &api.ImageMetadataTemplate{
		Template:   template,
		Properties: g.defFile.Template.Properties,
		When:       g.defFile.Template.When,
	}

	if len(g.defFile.Template.When) == 0 {
		img.Metadata.Templates[g.defFile.Path].When = []string{
			"create",
			"copy",
		}
	}

	return nil
}

// Run does nothing.
func (g *template) Run() error {
	return nil
}
