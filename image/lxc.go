package image

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	lxd "github.com/lxc/lxd/shared"
	"gopkg.in/flosch/pongo2.v3"

	"github.com/lxc/distrobuilder/shared"
)

const maxLXCCompatLevel = 5

// LXCImage represents a LXC image.
type LXCImage struct {
	sourceDir  string
	targetDir  string
	cacheDir   string
	definition shared.DefinitionImage
	target     shared.DefinitionTargetLXC
}

// NewLXCImage returns a LXCImage.
func NewLXCImage(sourceDir, targetDir, cacheDir string, definition shared.DefinitionImage,
	target shared.DefinitionTargetLXC) *LXCImage {
	img := LXCImage{
		sourceDir,
		targetDir,
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

	err = shared.Pack(filepath.Join(l.targetDir, "rootfs.tar"), "xz", l.sourceDir, ".")
	if err != nil {
		return err
	}

	return nil
}

func (l *LXCImage) createMetadata() error {
	metaDir := filepath.Join(l.cacheDir, "metadata")

	for _, c := range l.target.Config {
		// If not specified, create files up to ${maxLXCCompatLevel}
		if c.Before == 0 {
			c.Before = maxLXCCompatLevel + 1
		}
		for i := uint(1); i < maxLXCCompatLevel+1; i++ {
			// Bound checking
			if c.After < c.Before {
				if i <= c.After || i >= c.Before {
					continue
				}

			} else if c.After >= c.Before {
				if i <= c.After && i >= c.Before {
					continue
				}
			}

			switch c.Type {
			case "all":
				err := l.writeConfig(i, filepath.Join(metaDir, "config"), c.Content)
				if err != nil {
					return err
				}

				err = l.writeConfig(i, filepath.Join(metaDir, "config.user"), c.Content)
				if err != nil {
					return err
				}
			case "system":
				err := l.writeConfig(i, filepath.Join(metaDir, "config"), c.Content)
				if err != nil {
					return err
				}
			case "user":
				err := l.writeConfig(i, filepath.Join(metaDir, "config.user"), c.Content)
				if err != nil {
					return err
				}
			}
		}
	}

	err := l.writeMetadata(filepath.Join(metaDir, "create-message"),
		l.target.CreateMessage, false)
	if err != nil {
		return fmt.Errorf("Error writing 'create-message': %s", err)
	}

	err = l.writeMetadata(filepath.Join(metaDir, "expiry"),
		fmt.Sprint(shared.GetExpiryDate(time.Now(), l.definition.Expiry).Unix()),
		false)
	if err != nil {
		return fmt.Errorf("Error writing 'expiry': %s", err)
	}

	var excludesUser string

	if lxd.PathExists(filepath.Join(l.sourceDir, "dev")) {
		err := filepath.Walk(filepath.Join(l.sourceDir, "dev"),
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if info.Mode()&os.ModeDevice != 0 {
					excludesUser += fmt.Sprintf("%s\n",
						strings.TrimPrefix(path, l.sourceDir))
				}

				return nil
			})
		if err != nil {
			return fmt.Errorf("Error while walking /dev: %s", err)
		}
	}

	err = l.writeMetadata(filepath.Join(metaDir, "excludes-user"), excludesUser,
		false)
	if err != nil {
		return fmt.Errorf("Error writing 'excludes-user': %s", err)
	}

	return nil
}

func (l *LXCImage) packMetadata() error {
	files := []string{"create-message", "expiry", "excludes-user"}

	// Get all config and config.user files
	configs, err := filepath.Glob(filepath.Join(l.cacheDir, "metadata", "config*"))
	if err != nil {
		return err
	}

	for _, c := range configs {
		files = append(files, filepath.Base(c))
	}

	if lxd.PathExists(filepath.Join(l.cacheDir, "metadata", "templates")) {
		files = append(files, "templates")
	}

	err = shared.Pack(filepath.Join(l.targetDir, "meta.tar"), "xz",
		filepath.Join(l.cacheDir, "metadata"), files...)
	if err != nil {
		return fmt.Errorf("Failed to create metadata: %s", err)
	}

	return nil
}
func (l *LXCImage) writeMetadata(filename, content string, append bool) error {
	var file *os.File
	var err error

	if append {
		file, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	} else {
		file, err = os.Create(filename)
	}
	if err != nil {
		return err
	}
	defer file.Close()

	ctx := pongo2.Context{
		"image": l.definition,
	}

	out, err := shared.RenderTemplate(content, ctx)
	if err != nil {
		return err
	}

	_, err = file.WriteString(out + "\n")
	if err != nil {
		return err
	}

	return nil
}

func (l *LXCImage) writeConfig(compatLevel uint, filename, content string) error {
	// Only add suffix if it's not the latest compatLevel
	if compatLevel != maxLXCCompatLevel {
		filename = fmt.Sprintf("%s.%d", filename, compatLevel)
	}
	err := l.writeMetadata(filename, content, true)
	if err != nil {
		return fmt.Errorf("Error writing '%s': %s", filepath.Base(filename), err)
	}

	return nil
}
