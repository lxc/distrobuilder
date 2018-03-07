package generators

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lxc/lxd/shared/api"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

// HostnameGenerator represents the Hostname generator.
type HostnameGenerator struct{}

// RunLXC creates a hostname template.
func (g HostnameGenerator) RunLXC(cacheDir, sourceDir string, img *image.LXCImage,
	defFile shared.DefinitionFile) error {
	// Store original file
	err := StoreFile(cacheDir, sourceDir, defFile.Path)
	if err != nil {
		return err
	}

	// Create new hostname file
	file, err := os.Create(filepath.Join(sourceDir, defFile.Path))
	if err != nil {
		return err
	}
	defer file.Close()

	// Write LXC specific string to the hostname file
	_, err = file.WriteString("LXC_NAME\n")
	if err != nil {
		return fmt.Errorf("Failed to write to hostname file: %s", err)
	}

	// Add hostname path to LXC's templates file
	return img.AddTemplate(defFile.Path)
}

// RunLXD creates a hostname template.
func (g HostnameGenerator) RunLXD(cacheDir, sourceDir string, img *image.LXDImage,
	defFile shared.DefinitionFile) error {
	templateDir := filepath.Join(cacheDir, "templates")

	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(templateDir, "hostname.tpl"))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString("{{ container.name }}\n")
	if err != nil {
		return fmt.Errorf("Failed to write to hostname file: %s", err)
	}

	// Add to LXD templates
	img.Metadata.Templates[defFile.Path] = &api.ImageMetadataTemplate{
		Template: "hostname.tpl",
		When: []string{
			"create",
			"copy",
		},
	}

	return err
}
