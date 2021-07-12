package generators

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/flosch/pongo2"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

type dump struct {
	common
}

// RunLXC dumps content to a file.
func (g *dump) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {
	content := g.defFile.Content

	if g.defFile.Pongo {
		tpl, err := pongo2.FromString(g.defFile.Content)
		if err != nil {
			return err
		}

		content, err = tpl.Execute(pongo2.Context{"lxc": target})
		if err != nil {
			return err
		}
	}

	err := g.run(content)
	if err != nil {
		return err
	}

	if g.defFile.Templated {
		return img.AddTemplate(g.defFile.Path)
	}

	return nil
}

// RunLXD dumps content to a file.
func (g *dump) RunLXD(img *image.LXDImage, target shared.DefinitionTargetLXD) error {
	content := g.defFile.Content

	if g.defFile.Pongo {
		tpl, err := pongo2.FromString(g.defFile.Content)
		if err != nil {
			return err
		}

		content, err = tpl.Execute(pongo2.Context{"lxd": target})
		if err != nil {
			return err
		}
	}

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

	return updateFileAccess(file, g.defFile)
}
