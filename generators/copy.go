package generators

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

// CopyGenerator represents the Copy generator.
type CopyGenerator struct{}

// RunLXC copies a file to the container.
func (g CopyGenerator) RunLXC(cacheDir, sourceDir string, img *image.LXCImage,
	defFile shared.DefinitionFile) error {
	return g.Run(cacheDir, sourceDir, defFile)
}

// RunLXD copies a file to the container.
func (g CopyGenerator) RunLXD(cacheDir, sourceDir string, img *image.LXDImage,
	defFile shared.DefinitionFile) error {
	return g.Run(cacheDir, sourceDir, defFile)
}

// Run copies a file to the container.
func (g CopyGenerator) Run(cacheDir, sourceDir string,
	defFile shared.DefinitionFile) error {
	in, err := os.Open(defFile.Path)
	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("File '%s' doesn't exist", defFile.Path)
		}
		return err
	}
	defer in.Close()

	var dest string
	if defFile.Destination != "" {
		dest = filepath.Join(sourceDir, defFile.Destination)
	} else {
		dest = filepath.Join(sourceDir, defFile.Path)
	}
	// Let's make sure that we can create "the file"
	dir := filepath.Dir(dest)
	_, err = os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
	}
	if err != nil {
		return err
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	err = out.Chown(0, 0)
	if err != nil {
		return err
	}

	info, err := in.Stat()
	if err != nil {
		return err
	}
	err = out.Chmod(info.Mode())
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	return err
}
