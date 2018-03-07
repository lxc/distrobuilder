package generators

import (
	"io"
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/lxd/shared/api"
)

// HostsGenerator represents the hosts generator.
type HostsGenerator struct{}

// RunLXC creates a LXC specific entry in the hosts file.
func (g HostsGenerator) RunLXC(cacheDir, sourceDir string, img *image.LXCImage,
	defFile shared.DefinitionFile) error {
	// Store original file
	err := StoreFile(cacheDir, sourceDir, defFile.Path)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filepath.Join(sourceDir, defFile.Path),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Append hosts entry
	file.WriteString("127.0.0.1\tLXC_NAME\n")

	// Add hostname path to LXC's templates file
	return img.AddTemplate(defFile.Path)
}

// RunLXD creates a hosts template.
func (g HostsGenerator) RunLXD(cacheDir, sourceDir string, img *image.LXDImage,
	defFile shared.DefinitionFile) error {
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

	hostsFile, err := os.Open(filepath.Join(sourceDir, defFile.Path))
	if err != nil {
		return err
	}
	defer hostsFile.Close()

	// Copy old content, and append LXD specific entry
	io.Copy(file, hostsFile)
	file.WriteString("127.0.0.1\t{{ container.name }}\n")

	img.Metadata.Templates[defFile.Path] = &api.ImageMetadataTemplate{
		Template: "hosts.tpl",
		When: []string{
			"create",
			"copy",
		},
	}

	return err
}
