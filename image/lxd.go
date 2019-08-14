package image

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lxc/lxd/shared/api"
	"gopkg.in/yaml.v2"

	"github.com/lxc/distrobuilder/shared"
)

// A LXDImage represents a LXD image.
type LXDImage struct {
	sourceDir  string
	targetDir  string
	cacheDir   string
	Metadata   api.ImageMetadata
	definition shared.Definition
}

// NewLXDImage returns a LXDImage.
func NewLXDImage(sourceDir, targetDir, cacheDir string,
	definition shared.Definition) *LXDImage {
	return &LXDImage{
		sourceDir,
		targetDir,
		cacheDir,
		api.ImageMetadata{
			Properties: make(map[string]string),
			Templates:  make(map[string]*api.ImageMetadataTemplate),
		},
		definition,
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
		var fname string
		if l.definition.Image.Name != "" {
			// Use a custom name for the unified tarball.
			fname, _ = shared.RenderTemplate(l.definition.Image.Name, l.definition)
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
			filepath.Join(l.targetDir, "rootfs.squashfs"), "-noappend", "-comp",
			"xz", "-b", "1M", "-no-progress", "-no-recovery")
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

	l.Metadata.Architecture = l.definition.Image.Architecture
	l.Metadata.CreationDate = time.Now().UTC().Unix()
	l.Metadata.Properties["architecture"] = l.definition.Image.ArchitectureMapped
	l.Metadata.Properties["os"] = l.definition.Image.Distribution
	l.Metadata.Properties["release"] = l.definition.Image.Release
	l.Metadata.Properties["variant"] = l.definition.Image.Variant
	l.Metadata.Properties["serial"] = l.definition.Image.Serial

	l.Metadata.Properties["description"], err = shared.RenderTemplate(
		l.definition.Image.Description, l.definition)
	if err != nil {
		return err
	}

	l.Metadata.Properties["name"], err = shared.RenderTemplate(
		l.definition.Image.Name, l.definition)
	if err != nil {
		return err
	}

	l.Metadata.ExpiryDate = shared.GetExpiryDate(time.Now(),
		l.definition.Image.Expiry).Unix()

	return err
}
