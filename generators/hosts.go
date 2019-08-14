package generators

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"
)

// HostsGenerator represents the hosts generator.
type HostsGenerator struct{}

// RunLXC creates a LXC specific entry in the hosts file.
func (g HostsGenerator) RunLXC(cacheDir, sourceDir string, img *image.LXCImage,
	defFile shared.DefinitionFile) error {

	// Skip if the file doesn't exist
	if !lxd.PathExists(filepath.Join(sourceDir, defFile.Path)) {
		return nil
	}

	// Read the current content
	content, err := ioutil.ReadFile(filepath.Join(sourceDir, defFile.Path))
	if err != nil {
		return err
	}

	// Store original file
	err = StoreFile(cacheDir, sourceDir, defFile.Path)
	if err != nil {
		return err
	}

	// Replace hostname with placeholder
	content = []byte(strings.Replace(string(content), "distrobuilder", "LXC_NAME", -1))

	// Add a new line if needed
	if !strings.Contains(string(content), "LXC_NAME") {
		content = append([]byte("127.0.1.1\tLXC_NAME\n"), content...)
	}

	f, err := os.Create(filepath.Join(sourceDir, defFile.Path))
	if err != nil {
		return err
	}
	defer f.Close()

	// Overwrite the file
	_, err = f.Write(content)
	if err != nil {
		return err
	}

	// Add hostname path to LXC's templates file
	return img.AddTemplate(defFile.Path)
}

// RunLXD creates a hosts template.
func (g HostsGenerator) RunLXD(cacheDir, sourceDir string, img *image.LXDImage,
	defFile shared.DefinitionFile) error {

	// Skip if the file doesn't exist
	if !lxd.PathExists(filepath.Join(sourceDir, defFile.Path)) {
		return nil
	}

	templateDir := filepath.Join(cacheDir, "templates")

	// Create templates path
	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		return err
	}

	// Read the current content
	content, err := ioutil.ReadFile(filepath.Join(sourceDir, defFile.Path))
	if err != nil {
		return err
	}

	// Replace hostname with placeholder
	content = []byte(strings.Replace(string(content), "distrobuilder", "{{ container.name }}", -1))

	// Add a new line if needed
	if !strings.Contains(string(content), "{{ container.name }}") {
		content = append([]byte("127.0.1.1\t{{ container.name }}\n"), content...)
	}

	// Write the template
	err = ioutil.WriteFile(filepath.Join(templateDir, "hosts.tpl"), content, 0644)
	if err != nil {
		return err
	}

	img.Metadata.Templates[defFile.Path] = &api.ImageMetadataTemplate{
		Template: "hosts.tpl",
		When: []string{
			"create",
			"copy",
		},
	}

	return err
}

// Run does nothing.
func (g HostsGenerator) Run(cacheDir, sourceDir string,
	defFile shared.DefinitionFile) error {
	return nil
}
