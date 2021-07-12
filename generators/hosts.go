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

type hosts struct {
	common
}

// RunLXC creates a LXC specific entry in the hosts file.
func (g *hosts) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {

	// Skip if the file doesn't exist
	if !lxd.PathExists(filepath.Join(g.sourceDir, g.defFile.Path)) {
		return nil
	}

	// Read the current content
	content, err := ioutil.ReadFile(filepath.Join(g.sourceDir, g.defFile.Path))
	if err != nil {
		return err
	}

	// Replace hostname with placeholder
	content = []byte(strings.Replace(string(content), "distrobuilder", "LXC_NAME", -1))

	// Add a new line if needed
	if !strings.Contains(string(content), "LXC_NAME") {
		content = append([]byte("127.0.1.1\tLXC_NAME\n"), content...)
	}

	f, err := os.Create(filepath.Join(g.sourceDir, g.defFile.Path))
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
	return img.AddTemplate(g.defFile.Path)
}

// RunLXD creates a hosts template.
func (g *hosts) RunLXD(img *image.LXDImage, target shared.DefinitionTargetLXD) error {

	// Skip if the file doesn't exist
	if !lxd.PathExists(filepath.Join(g.sourceDir, g.defFile.Path)) {
		return nil
	}

	templateDir := filepath.Join(g.cacheDir, "templates")

	// Create templates path
	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		return err
	}

	// Read the current content
	content, err := ioutil.ReadFile(filepath.Join(g.sourceDir, g.defFile.Path))
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

	img.Metadata.Templates[g.defFile.Path] = &api.ImageMetadataTemplate{
		Template:   "hosts.tpl",
		Properties: g.defFile.Template.Properties,
		When:       g.defFile.Template.When,
	}

	if len(g.defFile.Template.When) == 0 {
		img.Metadata.Templates[g.defFile.Path].When = []string{
			"create",
			"copy",
		}
	}

	return err
}

// Run does nothing.
func (g *hosts) Run() error {
	return nil
}
