package generators

import (
	"os"
	"path/filepath"

	lxd "github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"
	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

type hostname struct {
	common
}

// RunLXC creates a hostname template.
func (g *hostname) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {

	// Skip if the file doesn't exist
	if !lxd.PathExists(filepath.Join(g.sourceDir, g.defFile.Path)) {
		return nil
	}

	// Create new hostname file
	file, err := os.Create(filepath.Join(g.sourceDir, g.defFile.Path))
	if err != nil {
		return errors.WithMessagef(err, "Failed to create file %q", filepath.Join(g.sourceDir, g.defFile.Path))
	}
	defer file.Close()

	// Write LXC specific string to the hostname file
	_, err = file.WriteString("LXC_NAME\n")
	if err != nil {
		return errors.WithMessage(err, "Failed to write to hostname file")
	}

	// Add hostname path to LXC's templates file
	err = img.AddTemplate(g.defFile.Path)
	if err != nil {
		return errors.WithMessage(err, "Failed to add template")
	}

	return nil
}

// RunLXD creates a hostname template.
func (g *hostname) RunLXD(img *image.LXDImage, target shared.DefinitionTargetLXD) error {

	// Skip if the file doesn't exist
	if !lxd.PathExists(filepath.Join(g.sourceDir, g.defFile.Path)) {
		return nil
	}

	templateDir := filepath.Join(g.cacheDir, "templates")

	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		return errors.WithMessagef(err, "Failed to create directory %q", templateDir)
	}

	file, err := os.Create(filepath.Join(templateDir, "hostname.tpl"))
	if err != nil {
		return errors.WithMessagef(err, "Failed to create file %q", filepath.Join(templateDir, "hostname.tpl"))
	}
	defer file.Close()

	_, err = file.WriteString("{{ container.name }}\n")
	if err != nil {
		return errors.WithMessage(err, "Failed to write to hostname file")
	}

	// Add to LXD templates
	img.Metadata.Templates[g.defFile.Path] = &api.ImageMetadataTemplate{
		Template:   "hostname.tpl",
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
func (g *hostname) Run() error {
	return nil
}
