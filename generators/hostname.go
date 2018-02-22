package generators

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/lxd/shared/api"
)

// HostnameGenerator represents the Hostname generator.
type HostnameGenerator struct{}

// CreateLXCData creates a hostname template.
func (g HostnameGenerator) CreateLXCData(cacheDir, path string, img *image.LXCImage) error {
	rootfs := filepath.Join(cacheDir, "rootfs")

	// Store original file
	err := StoreFile(cacheDir, path)
	if err != nil {
		return err
	}

	// Create new hostname file
	file, err := os.Create(filepath.Join(rootfs, path))
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
	return img.AddTemplate(path)
}

// CreateLXDData creates a hostname template.
func (g HostnameGenerator) CreateLXDData(cacheDir, path string, img *image.LXDImage) error {
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
	img.Metadata.Templates[path] = &api.ImageMetadataTemplate{
		Template: "hostname.tpl",
		When: []string{
			"create",
			"copy",
		},
	}

	return err
}
