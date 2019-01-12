package generators

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

// DumpGenerator represents the Remove generator.
type DumpGenerator struct{}

// RunLXC dumps content to a file.
func (g DumpGenerator) RunLXC(cacheDir, sourceDir string, img *image.LXCImage,
	defFile shared.DefinitionFile) error {
	err := g.Run(cacheDir, sourceDir, defFile)
	if err != nil {
		return err
	}

	if defFile.Templated {
		return img.AddTemplate(defFile.Path)
	}

	return nil
}

// RunLXD dumps content to a file.
func (g DumpGenerator) RunLXD(cacheDir, sourceDir string, img *image.LXDImage,
	defFile shared.DefinitionFile) error {
	return g.Run(cacheDir, sourceDir, defFile)
}

// Run dumps content to a file.
func (g DumpGenerator) Run(cacheDir, sourceDir string, defFile shared.DefinitionFile) error {
	return g.dumpFile(filepath.Join(sourceDir, defFile.Path), defFile.Content)
}

func (g DumpGenerator) dumpFile(path, content string) error {
	// Create any missing directory
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	// Open the target file (create if needed)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Append final new line if missing
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	// Write the content
	_, err = file.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}
