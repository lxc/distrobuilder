package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flosch/pongo2"
	"github.com/lxc/lxd/shared/api"
	"github.com/pkg/errors"

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
		return errors.Wrapf(err, "Failed to create directory %q", templateDir)
	}
	template := fmt.Sprintf("%s.tpl", g.defFile.Name)

	file, err := os.Create(filepath.Join(templateDir, template))
	if err != nil {
		return errors.Wrapf(err, "Failed to create file %q", filepath.Join(templateDir, template))
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
			return errors.Wrap(err, "Failed to parse template")
		}

		content, err = tpl.Execute(pongo2.Context{"lxd": target})
		if err != nil {
			return errors.Wrapf(err, "Failed to execute template")
		}
	}

	_, err = file.WriteString(content)
	if err != nil {
		return errors.Wrapf(err, "Failed to write to content to %s template", g.defFile.Name)
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
