package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/lxd/shared/api"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

// TemplateGenerator represents the Template generator.
type TemplateGenerator struct{}

// RunLXC dumps content to a file.
func (g TemplateGenerator) RunLXC(cacheDir, sourceDir string, img *image.LXCImage,
	defFile shared.DefinitionFile) error {
	// no template support for LXC, ignoring generator
	return nil
}

// RunLXD dumps content to a file.
func (g TemplateGenerator) RunLXD(cacheDir, sourceDir string, img *image.LXDImage,
	defFile shared.DefinitionFile) error {
	templateDir := filepath.Join(cacheDir, "templates")

	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		return err
	}
	template := fmt.Sprintf("%s.tpl", defFile.Name)

	file, err := os.Create(filepath.Join(templateDir, template))
	if err != nil {
		return err
	}

	defer file.Close()

	// Append final new line if missing
	if !strings.HasSuffix(defFile.Content, "\n") {
		defFile.Content += "\n"
	}

	_, err = file.WriteString(defFile.Content)
	if err != nil {
		return fmt.Errorf("Failed to write to content to %s template: %s", defFile.Name, err)
	}

	// Add to LXD templates
	img.Metadata.Templates[defFile.Path] = &api.ImageMetadataTemplate{
		Template:   template,
		Properties: defFile.Template.Properties,
		When:       defFile.Template.When,
	}

	if len(defFile.Template.When) == 0 {
		img.Metadata.Templates[defFile.Path].When = []string{
			"create",
			"copy",
		}
	}

	return err
}

// Run does nothing.
func (g TemplateGenerator) Run(cacheDir, sourceDir string,
	defFile shared.DefinitionFile) error {
	return nil
}
