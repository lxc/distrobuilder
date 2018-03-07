package image

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/osarch"
	"gopkg.in/flosch/pongo2.v3"
	"gopkg.in/yaml.v2"

	"github.com/lxc/distrobuilder/shared"
)

// A LXDImage represents a LXD image.
type LXDImage struct {
	sourceDir    string
	targetDir    string
	cacheDir     string
	creationDate time.Time
	Metadata     api.ImageMetadata
	definition   shared.DefinitionImage
}

// NewLXDImage returns a LXDImage.
func NewLXDImage(sourceDir, targetDir, cacheDir string,
	imageDef shared.DefinitionImage) *LXDImage {
	return &LXDImage{
		sourceDir,
		targetDir,
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
func (l *LXDImage) Build(unified bool, compression string) error {
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

	paths := []string{"metadata.yaml"}

	// Only include templates directory in the tarball if it's present.
	info, err := os.Stat(filepath.Join(l.cacheDir, "templates"))
	if err == nil && info.IsDir() {
		paths = append(paths, "templates")
	}

	if unified {
		ctx := pongo2.Context{
			"image":         l.definition,
			"creation_date": l.creationDate.Format("20060201_1504"),
		}

		var fname string
		if l.definition.Name != "" {
			// Use a custom name for the unified tarball.
			fname, _ = shared.RenderTemplate(l.definition.Name, ctx)
		} else {
			// Default name for the unified tarball.
			fname = "lxd"
		}

		// Add the rootfs to the tarball, prefix all files with "rootfs"
		err = shared.Pack(filepath.Join(l.targetDir, fmt.Sprintf("%s.tar", fname)),
			"", l.sourceDir, "--transform", "s,^./,rootfs/,", ".")
		if err != nil {
			return err
		}

		// Add the metadata to the tarball which is located in the cache directory
		err = shared.PackUpdate(filepath.Join(l.targetDir, fmt.Sprintf("%s.tar", fname)),
			compression, l.cacheDir, paths...)
		if err != nil {
			return err
		}

	} else {
		// Create rootfs as squashfs.
		err = shared.RunCommand("mksquashfs", l.sourceDir,
			filepath.Join(l.targetDir, "rootfs.squashfs"), "-noappend")
		if err != nil {
			return err
		}

		// Create metadata tarball.
		err = shared.Pack(filepath.Join(l.targetDir, "lxd.tar"), compression,
			l.cacheDir, paths...)
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

	// Use proper architecture name from now on.
	l.definition.Arch = arch

	l.Metadata.Architecture = l.definition.Arch
	l.Metadata.CreationDate = l.creationDate.Unix()
	l.Metadata.Properties["architecture"] = l.definition.Arch
	l.Metadata.Properties["os"] = l.definition.Distribution
	l.Metadata.Properties["release"] = l.definition.Release

	ctx := pongo2.Context{
		"image":         l.definition,
		"creation_date": l.creationDate.Format("20060201_1504"),
	}

	l.Metadata.Properties["description"], err = shared.RenderTemplate(l.definition.Description, ctx)
	if err != err {
		return nil
	}

	l.Metadata.Properties["name"], err = shared.RenderTemplate(l.definition.Name, ctx)
	if err != nil {
		return err
	}

	l.Metadata.ExpiryDate = shared.GetExpiryDate(l.creationDate, l.definition.Expiry).Unix()

	return err
}
