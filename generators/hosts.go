package generators

import (
	"io"
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/lxd/shared/api"
)

// HostsGenerator represents the hosts generator.
type HostsGenerator struct{}

// CreateLXCData creates a LXC specific entry in the hosts file.
func (g HostsGenerator) CreateLXCData(cacheDir, sourceDir, path string, img *image.LXCImage) error {
	// Store original file
	err := StoreFile(cacheDir, sourceDir, path)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filepath.Join(sourceDir, path),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Append hosts entry
	file.WriteString("127.0.0.1\tLXC_NAME\n")

	// Add hostname path to LXC's templates file
	return img.AddTemplate(path)
}

// CreateLXDData creates a hosts template.
func (g HostsGenerator) CreateLXDData(cacheDir, sourceDir, path string, img *image.LXDImage) error {
	templateDir := filepath.Join(cacheDir, "templates")

	// Create templates path
	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		return err
	}

	// Create hosts template
	file, err := os.Create(filepath.Join(templateDir, "hosts.tpl"))
	if err != nil {
		return err
	}
	defer file.Close()

	hostsFile, err := os.Open(filepath.Join(sourceDir, path))
	if err != nil {
		return err
	}
	defer hostsFile.Close()

	// Copy old content, and append LXD specific entry
	io.Copy(file, hostsFile)
	file.WriteString("127.0.0.1\t{{ container.name }}\n")

	img.Metadata.Templates[path] = &api.ImageMetadataTemplate{
		Template: "hosts.tpl",
		When: []string{
			"create",
			"copy",
		},
	}

	return err
}
