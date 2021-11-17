package image

import (
	"context"
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
	ctx        context.Context
}

// NewLXDImage returns a LXDImage.
func NewLXDImage(ctx context.Context, sourceDir, targetDir, cacheDir string,
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
		ctx,
	}
}

// Build creates a LXD image.
func (l *LXDImage) Build(unified bool, compression string, vm bool) (string, string, error) {
	err := l.createMetadata()
	if err != nil {
		return "", "", fmt.Errorf("Failed to create metadata: %w", err)
	}

	file, err := os.Create(filepath.Join(l.cacheDir, "metadata.yaml"))
	if err != nil {
		return "", "", fmt.Errorf("Failed to create metadata.yaml: %w", err)
	}
	defer file.Close()

	data, err := yaml.Marshal(l.Metadata)
	if err != nil {
		return "", "", fmt.Errorf("Failed to marshal yaml: %w", err)
	}

	_, err = file.Write(data)
	if err != nil {
		return "", "", fmt.Errorf("Failed to write metadata: %w", err)
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
		err = shared.RunCommand(l.ctx, "qemu-img", "convert", "-c", "-O", "qcow2", "-o", "compat=0.10",
			rawImage,
			qcowImage)
		if err != nil {
			return "", "", fmt.Errorf("Failed to create qcow2 image %q: %w", qcowImage, err)
		}
		defer func() {
			os.RemoveAll(rawImage)
		}()
	}

	imageFile := ""
	rootfsFile := ""

	if unified {
		targetTarball := filepath.Join(l.targetDir, fmt.Sprintf("%s.tar", fname))

		if vm {
			// Rename image to rootfs.img
			err = os.Rename(qcowImage, filepath.Join(filepath.Dir(qcowImage), "rootfs.img"))
			if err != nil {
				return "", "", fmt.Errorf("Failed to rename image %q -> %q: %w", qcowImage, filepath.Join(filepath.Dir(qcowImage), "rootfs.img"), err)
			}

			_, err = shared.Pack(l.ctx, targetTarball, "", l.cacheDir, "rootfs.img")
		} else {
			// Add the rootfs to the tarball, prefix all files with "rootfs".
			// We intentionally don't set any compression here, as PackUpdate (further down) cannot deal with compressed tarballs.
			_, err = shared.Pack(l.ctx, targetTarball,
				"", l.sourceDir, "--transform", "s,^./,rootfs/,", ".")
		}
		if err != nil {
			return "", "", fmt.Errorf("Failed to pack tarball %q: %w", targetTarball, err)
		}
		defer func() {
			if vm {
				os.RemoveAll(qcowImage)
			}
		}()

		// Add the metadata to the tarball which is located in the cache directory
		imageFile, err = shared.PackUpdate(l.ctx, targetTarball, compression, l.cacheDir, paths...)
		if err != nil {
			return "", "", fmt.Errorf("Failed to add metadata to tarball %q: %w", targetTarball, err)
		}
	} else {
		if vm {
			rootfsFile = filepath.Join(l.targetDir, "disk.qcow2")

			err = shared.Copy(qcowImage, rootfsFile)
		} else {
			rootfsFile = filepath.Join(l.targetDir, "rootfs.squashfs")

			// Create rootfs as squashfs.
			err = shared.RunCommand(l.ctx, "mksquashfs", l.sourceDir,
				rootfsFile, "-noappend", "-comp",
				compression, "-b", "1M", "-no-progress", "-no-recovery")
		}
		if err != nil {
			return "", "", fmt.Errorf("Failed to create squashfs or copy image: %w", err)
		}

		// Create metadata tarball.
		imageFile, err = shared.Pack(l.ctx, filepath.Join(l.targetDir, "lxd.tar"), compression,
			l.cacheDir, paths...)
		if err != nil {
			return "", "", fmt.Errorf("Failed to create metadata tarball: %w", err)
		}
	}

	return imageFile, rootfsFile, nil
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
		return fmt.Errorf("Failed to render template: %w", err)
	}

	l.Metadata.Properties["name"], err = shared.RenderTemplate(
		l.definition.Image.Name, l.definition)
	if err != nil {
		return fmt.Errorf("Failed to render template: %w", err)
	}

	l.Metadata.ExpiryDate = shared.GetExpiryDate(time.Now(),
		l.definition.Image.Expiry).Unix()

	return nil
}
