package image

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/osarch"
	pongo2 "gopkg.in/flosch/pongo2.v3"
	yaml "gopkg.in/yaml.v2"
)

// A LXDImage represents a LXD image.
type LXDImage struct {
	cacheDir     string
	creationDate time.Time
	Metadata     api.ImageMetadata
	definition   shared.DefinitionImage
}

// NewLXDImage returns a LXDImage.
func NewLXDImage(cacheDir string, imageDef shared.DefinitionImage) *LXDImage {
	return &LXDImage{
		cacheDir,
		time.Now(),
		api.ImageMetadata{
			Properties: make(map[string]string),
			Templates:  make(map[string]*api.ImageMetadataTemplate),
		},
		imageDef,
	}
}

// Build creates a LXD image.
func (l *LXDImage) Build(unified bool) error {
	err := l.createMetadata()
	if err != nil {
		return nil
	}

	file, err := os.Create(filepath.Join(l.cacheDir, "metadata.yaml"))
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := yaml.Marshal(l.Metadata)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("Failed to write metadata: %s", err)
	}

	if unified {
		var fname string
		paths := []string{"rootfs", "templates", "metadata.yaml"}

		ctx := pongo2.Context{
			"image":         l.definition,
			"creation_date": l.creationDate,
		}

		if l.definition.Name != "" {
			fname, _ = renderTemplate(l.definition.Name, ctx)
		} else {
			fname = "lxd"
		}

		err = shared.Pack(fmt.Sprintf("%s.tar.xz", fname), l.cacheDir, paths...)
		if err != nil {
			return err
		}
	} else {
		err = shared.RunCommand("mksquashfs", filepath.Join(l.cacheDir, "rootfs"),
			"rootfs.squashfs", "-noappend")
		if err != nil {
			return err
		}

		err = shared.Pack("lxd.tar.xz", l.cacheDir, "templates", "metadata.yaml")
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *LXDImage) createMetadata() error {
	var err error

	// Get the arch ID of the provided architecture.
	ID, err := osarch.ArchitectureId(l.definition.Arch)
	if err != nil {
		return err
	}

	// Get the "proper" name of the architecture.
	arch, err := osarch.ArchitectureName(ID)
	if err != nil {
		return err
	}

	l.Metadata.Architecture = arch
	l.Metadata.CreationDate = l.creationDate.Unix()
	l.Metadata.Properties["architecture"] = arch
	l.Metadata.Properties["os"] = l.definition.Distribution
	l.Metadata.Properties["release"] = l.definition.Release

	ctx := pongo2.Context{
		"image":         l.definition,
		"creation_date": l.creationDate,
	}

	l.Metadata.Properties["description"], err = renderTemplate(l.definition.Description, ctx)
	if err != err {
		return nil
	}

	l.Metadata.Properties["name"], err = renderTemplate(l.definition.Name, ctx)
	if err != nil {
		return err
	}

	l.Metadata.ExpiryDate = shared.GetExpiryDate(l.creationDate, l.definition.Expiry).Unix()

	return err
}
