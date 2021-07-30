package image

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lxc/lxd/shared/api"
	"github.com/pkg/errors"
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
func (l *LXDImage) Build(unified bool, compression string, vm bool) error {
	err := l.createMetadata()
	if err != nil {
		return errors.Wrap(err, "Failed to create metadata")
	}

	file, err := os.Create(filepath.Join(l.cacheDir, "metadata.yaml"))
	if err != nil {
		return errors.Wrap(err, "Failed to create metadata.yaml")
	}
	defer file.Close()

	data, err := yaml.Marshal(l.Metadata)
	if err != nil {
		return errors.Wrap(err, "Failed to marshal yaml")
	}

	_, err = file.Write(data)
	if err != nil {
		return errors.Wrap(err, "Failed to write metadata")
	}

	paths := []string{"metadata.yaml"}

	// Only include templates directory in the tarball if it's present.
	info, err := os.Stat(filepath.Join(l.cacheDir, "templates"))
	if err == nil && info.IsDir() {
		paths = append(paths, "templates")
	}

	var fname string
	if l.definition.Image.Name != "" {
		// Use a custom name for the unified tarball.
		fname, _ = shared.RenderTemplate(l.definition.Image.Name, l.definition)
	} else {
		// Default name for the unified tarball.
		fname = "lxd"
	}

	rawImage := filepath.Join(l.cacheDir, fmt.Sprintf("%s.raw", fname))
	qcowImage := filepath.Join(l.cacheDir, fmt.Sprintf("%s.img", fname))

	if vm {
		// Create compressed qcow2 image.
		err = shared.RunCommand("qemu-img", "convert", "-c", "-O", "qcow2", "-o", "compat=0.10",
			rawImage,
			qcowImage)
		if err != nil {
			return errors.Wrapf(err, "Failed to create qcow2 image %q", qcowImage)
		}
		defer func() {
			os.RemoveAll(rawImage)
		}()
	}

	if unified {
		targetTarball := filepath.Join(l.targetDir, fmt.Sprintf("%s.tar", fname))

		if vm {
			// Rename image to rootfs.img
			err = os.Rename(qcowImage, filepath.Join(filepath.Dir(qcowImage), "rootfs.img"))
			if err != nil {
				return errors.Wrapf(err, "Failed to rename image %q -> %q", qcowImage, filepath.Join(filepath.Dir(qcowImage), "rootfs.img"))
			}

			err = shared.Pack(targetTarball, "", l.cacheDir, "rootfs.img")
		} else {
			// Add the rootfs to the tarball, prefix all files with "rootfs"
			err = shared.Pack(targetTarball,
				compression, l.sourceDir, "--transform", "s,^./,rootfs/,", ".")
		}
		if err != nil {
			return errors.Wrapf(err, "Failed to pack tarball %q", targetTarball)
		}
		defer func() {
			if vm {
				os.RemoveAll(qcowImage)
			}
		}()

		// Add the metadata to the tarball which is located in the cache directory
		err = shared.PackUpdate(targetTarball, compression, l.cacheDir, paths...)
		if err != nil {
			return errors.Wrapf(err, "Failed to add metadata to tarball %q", targetTarball)
		}
	} else {
		if vm {
			err = shared.Copy(qcowImage, filepath.Join(l.targetDir, "disk.qcow2"))
		} else {
			// Create rootfs as squashfs.
			err = shared.RunCommand("mksquashfs", l.sourceDir,
				filepath.Join(l.targetDir, "rootfs.squashfs"), "-noappend", "-comp",
				compression, "-b", "1M", "-no-progress", "-no-recovery")
		}
		if err != nil {
			return errors.Wrap(err, "Failed to create squashfs or copy image")
		}

		// Create metadata tarball.
		err = shared.Pack(filepath.Join(l.targetDir, "lxd.tar"), compression,
			l.cacheDir, paths...)
		if err != nil {
			return errors.Wrap(err, "Failed to create metadata tarball")
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
		return errors.Wrap(err, "Failed to render template")
	}

	l.Metadata.Properties["name"], err = shared.RenderTemplate(
		l.definition.Image.Name, l.definition)
	if err != nil {
		return errors.Wrap(err, "Failed to render template")
	}

	l.Metadata.ExpiryDate = shared.GetExpiryDate(time.Now(),
		l.definition.Image.Expiry).Unix()

	return nil
}
