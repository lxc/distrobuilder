package image

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	lxd "github.com/lxc/lxd/shared"
	pongo2 "gopkg.in/flosch/pongo2.v3"

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

	err = l.writeMetadata(filepath.Join(metaDir, "expiry"),
		fmt.Sprint(shared.GetExpiryDate(time.Now(), l.definition.Expiry).Unix()))
	if err != nil {
		return fmt.Errorf("Error writing 'expiry': %s", err)
	}

	var excludesUser string

	if lxd.PathExists(filepath.Join(l.cacheDir, "rootfs", "dev")) {
		err := filepath.Walk(filepath.Join(l.cacheDir, "rootfs", "dev"),
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if info.Mode()&os.ModeDevice != 0 {
					excludesUser += fmt.Sprintf("%s\n",
						strings.TrimPrefix(path, filepath.Join(l.cacheDir, "rootfs")))
				}

				return nil
			})
		if err != nil {
			return fmt.Errorf("Error while walking /dev: %s", err)
		}
	}

	err = l.writeMetadata(filepath.Join(metaDir, "excludes-user"), excludesUser)
	if err != nil {
		return fmt.Errorf("Error writing 'excludes-user': %s", err)
	}

	return nil
}

func (l *LXCImage) packMetadata() error {
	files := []string{"config", "config-user", "create-message", "expiry",
		"excludes-user"}

	if lxd.PathExists(filepath.Join(l.cacheDir, "metadata", "templates")) {
		files = append(files, "templates")
	}

	err := shared.Pack("meta.tar.xz", filepath.Join(l.cacheDir, "metadata"), files...)
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

	ctx := pongo2.Context{
		"image": l.definition,
	}

	out, err := renderTemplate(content, ctx)
	if err != nil {
		return err
	}

	_, err = file.WriteString(out)
	if err != nil {
		return err
	}

	return nil
}
