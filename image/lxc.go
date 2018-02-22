package image

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxc/distrobuilder/shared"
)

// LXCImage represents a LXC image.
type LXCImage struct {
	cacheDir   string
	definition shared.DefinitionImage
	target     shared.DefinitionTargetLXC
}

// NewLXCImage returns a LXCImage.
func NewLXCImage(cacheDir string, definition shared.DefinitionImage,
	target shared.DefinitionTargetLXC) *LXCImage {
	img := LXCImage{
		cacheDir,
		definition,
		target,
	}

	// create metadata directory
	err := os.MkdirAll(filepath.Join(cacheDir, "metadata"), 0755)
	if err != nil {
		return nil
	}

	return &img
}

// AddTemplate adds an entry to the templates file.
func (l *LXCImage) AddTemplate(path string) error {
	metaDir := filepath.Join(l.cacheDir, "metadata")

	file, err := os.OpenFile(filepath.Join(metaDir, "templates"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf("%v\n", path))
	if err != nil {
		return fmt.Errorf("Failed to write to template file: %s", err)
	}

	return nil
}

// Build creates a LXC image.
func (l *LXCImage) Build() error {
	err := l.createMetadata()
	if err != nil {
		return err
	}

	err = l.packMetadata()
	if err != nil {
		return err
	}

	err = shared.Pack("rootfs.tar.xz", l.cacheDir, "rootfs")
	if err != nil {
		return err
	}

	return nil
}

func (l *LXCImage) createMetadata() error {
	metaDir := filepath.Join(l.cacheDir, "metadata")

	err := l.writeMetadata(filepath.Join(metaDir, "config"), l.target.Config)
	if err != nil {
		return fmt.Errorf("Error writing 'config': %s", err)
	}

	err = l.writeMetadata(filepath.Join(metaDir, "config-user"), l.target.ConfigUser)
	if err != nil {
		return fmt.Errorf("Error writing 'config-user': %s", err)
	}

	err = l.writeMetadata(filepath.Join(metaDir, "create-message"), l.target.CreateMessage)
	if err != nil {
		return fmt.Errorf("Error writing 'create-message': %s", err)
	}

	err = l.writeMetadata(filepath.Join(metaDir, "expiry"), string(time.Now().Unix()))
	if err != nil {
		return fmt.Errorf("Error writing 'expiry': %s", err)
	}
	var excludesUser string

	filepath.Walk(filepath.Join(l.cacheDir, "rootfs", "dev"),
		func(path string, info os.FileInfo, err error) error {
			if info.Mode()&os.ModeDevice != 0 {
				excludesUser += fmt.Sprintf("%s\n",
					strings.TrimPrefix(path, filepath.Join(l.cacheDir, "rootfs")))
			}

			return nil
		})

	err = l.writeMetadata(filepath.Join(metaDir, "excludes-user"), excludesUser)
	if err != nil {
		return fmt.Errorf("Error writing 'excludes-user': %s", err)
	}

	return nil
}

func (l *LXCImage) packMetadata() error {
	err := shared.Pack("meta.tar.xz", filepath.Join(l.cacheDir, "metadata"), "config",
		"config-user", "create-message", "expiry", "templates", "excludes-user")
	if err != nil {
		return fmt.Errorf("Failed to create metadata: %s", err)
	}

	return nil
}
func (l *LXCImage) writeMetadata(filename, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}
