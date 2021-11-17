package image

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

const maxLXCCompatLevel = 5

// LXCImage represents a LXC image.
type LXCImage struct {
	sourceDir  string
	targetDir  string
	cacheDir   string
	definition shared.Definition
	ctx        context.Context
}

// NewLXCImage returns a LXCImage.
func NewLXCImage(ctx context.Context, sourceDir, targetDir, cacheDir string, definition shared.Definition) *LXCImage {
	img := LXCImage{
		sourceDir,
		targetDir,
		cacheDir,
		definition,
		ctx,
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
		return fmt.Errorf("Failed to open file %q: %w", filepath.Join(metaDir, "templates"), err)
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf("%v\n", path))
	if err != nil {
		return fmt.Errorf("Failed to write to template file: %w", err)
	}

	return nil
}

// Build creates a LXC image.
func (l *LXCImage) Build(compression string) error {
	err := l.createMetadata()
	if err != nil {
		return fmt.Errorf("Failed to create metadata: %w", err)
	}

	err = l.packMetadata()
	if err != nil {
		return fmt.Errorf("Failed to pack metadata: %w", err)
	}

	_, err = shared.Pack(l.ctx, filepath.Join(l.targetDir, "rootfs.tar"), compression, l.sourceDir, ".")
	if err != nil {
		return fmt.Errorf("Failed to pack %q: %w", filepath.Join(l.targetDir, "rootfs.tar"), err)
	}

	return nil
}

func (l *LXCImage) createMetadata() error {
	metaDir := filepath.Join(l.cacheDir, "metadata")

	for _, c := range l.definition.Targets.LXC.Config {
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
					return fmt.Errorf("Failed to write config %q: %w", filepath.Join(metaDir, "config"), err)
				}

				err = l.writeConfig(i, filepath.Join(metaDir, "config-user"), c.Content)
				if err != nil {
					return fmt.Errorf("Failed to write config %q: %w", filepath.Join(metaDir, "config-user"), err)
				}
			case "system":
				err := l.writeConfig(i, filepath.Join(metaDir, "config"), c.Content)
				if err != nil {
					return fmt.Errorf("Failed to write config %q: %w", filepath.Join(metaDir, "config"), err)
				}
			case "user":
				err := l.writeConfig(i, filepath.Join(metaDir, "config-user"), c.Content)
				if err != nil {
					return fmt.Errorf("Failed to write config %q: %w", filepath.Join(metaDir, "config-user"), err)
				}
			}
		}
	}

	err := l.writeMetadata(filepath.Join(metaDir, "create-message"),
		l.definition.Targets.LXC.CreateMessage, false)
	if err != nil {
		return fmt.Errorf("Error writing 'create-message': %w", err)
	}

	err = l.writeMetadata(filepath.Join(metaDir, "expiry"),
		fmt.Sprint(shared.GetExpiryDate(time.Now(), l.definition.Image.Expiry).Unix()),
		false)
	if err != nil {
		return fmt.Errorf("Error writing 'expiry': %w", err)
	}

	var excludesUser string

	if lxd.PathExists(filepath.Join(l.sourceDir, "dev")) {
		err := filepath.Walk(filepath.Join(l.sourceDir, "dev"),
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if info.Mode()&os.ModeDevice != 0 {
					excludesUser += fmt.Sprintf(".%s\n",
						strings.TrimPrefix(path, l.sourceDir))
				}

				return nil
			})
		if err != nil {
			return fmt.Errorf("Error while walking /dev: %w", err)
		}
	}

	err = l.writeMetadata(filepath.Join(metaDir, "excludes-user"), excludesUser, false)
	if err != nil {
		return fmt.Errorf("Error writing 'excludes-user': %w", err)
	}

	return nil
}

func (l *LXCImage) packMetadata() error {
	files := []string{"create-message", "expiry", "excludes-user"}

	// Get all config and config-user files
	configs, err := filepath.Glob(filepath.Join(l.cacheDir, "metadata", "config*"))
	if err != nil {
		return fmt.Errorf("Failed to match file pattern: %w", err)
	}

	for _, c := range configs {
		files = append(files, filepath.Base(c))
	}

	if lxd.PathExists(filepath.Join(l.cacheDir, "metadata", "templates")) {
		files = append(files, "templates")
	}

	_, err = shared.Pack(l.ctx, filepath.Join(l.targetDir, "meta.tar"), "xz",
		filepath.Join(l.cacheDir, "metadata"), files...)
	if err != nil {
		return fmt.Errorf("Failed to create metadata: %w", err)
	}

	return nil
}
func (l *LXCImage) writeMetadata(filename, content string, appendContent bool) error {
	var file *os.File
	var err error

	// Open the file either in append or create mode
	if appendContent {
		file, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("Failed to open file %q: %w", filename, err)
		}
	} else {
		file, err = os.Create(filename)
		if err != nil {
			return fmt.Errorf("Failed to create file %q: %w", filename, err)
		}
	}
	defer file.Close()

	out, err := shared.RenderTemplate(content, l.definition)
	if err != nil {
		return fmt.Errorf("Failed to render template: %w", err)
	}

	// Append final new line if missing
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}

	// Write the content
	_, err = file.WriteString(out)
	if err != nil {
		return fmt.Errorf("Failed to write string: %w", err)
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
		return fmt.Errorf("Error writing '%s': %w", filepath.Base(filename), err)
	}

	return nil
}
