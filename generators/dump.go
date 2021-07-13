package generators

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/flosch/pongo2"
	"github.com/pkg/errors"

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
			return errors.Wrap(err, "Failed to parse template")
		}

		content, err = tpl.Execute(pongo2.Context{"lxc": target})
		if err != nil {
			return errors.Wrap(err, "Failed to execute template")
		}
	}

	err := g.run(content)
	if err != nil {
		return errors.Wrap(err, "Failed to dump content")
	}

	if g.defFile.Templated {
		err = img.AddTemplate(g.defFile.Path)
		if err != nil {
			return errors.Wrap(err, "Failed to add template")
		}
	}

	return nil
}

// RunLXD dumps content to a file.
func (g *dump) RunLXD(img *image.LXDImage, target shared.DefinitionTargetLXD) error {
	content := g.defFile.Content

	if g.defFile.Pongo {
		tpl, err := pongo2.FromString(g.defFile.Content)
		if err != nil {
			return errors.Wrap(err, "Failed to parse template")
		}

		content, err = tpl.Execute(pongo2.Context{"lxd": target})
		if err != nil {
			return errors.Wrapf(err, "Failed to execute template")
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
		return errors.Wrapf(err, "Failed to create directory %q", filepath.Dir(path))
	}

	// Open the target file (create if needed)
	file, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "Failed to create file %q", path)
	}
	defer file.Close()

	// Append final new line if missing
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	// Write the content
	_, err = file.WriteString(content)
	if err != nil {
		return errors.Wrapf(err, "Failed to write string to file %q", path)
	}

	err = updateFileAccess(file, g.defFile)
	if err != nil {
		return errors.Wrapf(err, "Failed to update file access of %q", path)
	}

	return nil
}
