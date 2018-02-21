package generators

import (
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/image"
)

// HostnameGenerator represents the Hostname generator.
type HostnameGenerator struct{}

// CreateLXCData creates a hostname template.
func (g HostnameGenerator) CreateLXCData(cacheDir, path string, img *image.LXCImage) error {
	rootfs := filepath.Join(cacheDir, "rootfs")

	// store original file
	err := StoreFile(cacheDir, path)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(rootfs, path))
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString("LXC_NAME\n")

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

	file.WriteString("{{ container.name }}\n")

	img.Metadata.Templates[path] = image.LXDMetadataTemplate{
		Template: "hostname.tpl",
		When: []string{
			"create",
			"copy",
		},
	}

	return err
}
