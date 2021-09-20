package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

type dump struct {
	common
}

// RunLXC dumps content to a file.
func (g *dump) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {
	content := g.defFile.Content

	err := g.run(content)
	if err != nil {
		return fmt.Errorf("Failed to dump content: %w", err)
	}

	if g.defFile.Templated {
		err = img.AddTemplate(g.defFile.Path)
		if err != nil {
			return fmt.Errorf("Failed to add template: %w", err)
		}
	}

	return nil
}

// RunLXD dumps content to a file.
func (g *dump) RunLXD(img *image.LXDImage, target shared.DefinitionTargetLXD) error {
	content := g.defFile.Content

	return g.run(content)
}

// Run dumps content to a file.
func (g *dump) Run() error {
	return g.run(g.defFile.Content)
}

func (g *dump) run(content string) error {
	path := filepath.Join(g.sourceDir, g.defFile.Path)

	// Create any missing directory
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", filepath.Dir(path), err)
	}

	// Open the target file (create if needed)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("Failed to create file %q: %w", path, err)
	}
	defer file.Close()

	// Append final new line if missing
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	// Write the content
	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("Failed to write string to file %q: %w", path, err)
	}

	err = updateFileAccess(file, g.defFile)
	if err != nil {
		return fmt.Errorf("Failed to update file access of %q: %w", path, err)
	}

	return nil
}
