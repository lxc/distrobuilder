package generators

import (
	"io"
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/image"
)

// HostsGenerator represents the hosts generator.
type HostsGenerator struct{}

// CreateLXCData creates a LXC specific entry in the hosts file.
func (g HostsGenerator) CreateLXCData(cacheDir, path string, img *image.LXCImage) error {
	rootfs := filepath.Join(cacheDir, "rootfs")

	// store original file
	err := StoreFile(cacheDir, path)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filepath.Join(rootfs, path),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString("127.0.0.1\tLXC_NAME\n")

	return img.AddTemplate(path)
}

// CreateLXDData creates a hosts template.
func (g HostsGenerator) CreateLXDData(cacheDir, path string, img *image.LXDImage) error {
	templateDir := filepath.Join(cacheDir, "templates")

	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(templateDir, "hosts.tpl"))
	if err != nil {
		return err
	}
	defer file.Close()

	hostsFile, err := os.Open(filepath.Join(cacheDir, "rootfs", path))
	if err != nil {
		return err
	}
	defer hostsFile.Close()

	io.Copy(file, hostsFile)
	file.WriteString("127.0.0.1\t{{ container.name }}\n")

	img.Metadata.Templates[path] = image.LXDMetadataTemplate{
		Template: "hostname.tpl",
		When: []string{
			"create",
			"copy",
		},
	}

	return err
}
