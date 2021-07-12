package generators

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
)

type copy struct {
	common
}

// RunLXC copies a file to the container.
func (g *copy) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {
	return g.Run()
}

// RunLXD copies a file to the container.
func (g *copy) RunLXD(img *image.LXDImage, target shared.DefinitionTargetLXD) error {
	return g.Run()
}

// Run copies a file to the container.
func (g *copy) Run() error {
	// First check if the input is a file or a directory.
	// Then check whether the destination finishes in a "/" or not
	// Afterwards, the rules for copying can be applied. See doc/generators.md

	// Set the name of the destination file to the input file
	// relative to the root if destination file is missing
	var destPath, srcPath string
	var files []string
	srcPath = g.defFile.Source
	destPath = filepath.Join(g.sourceDir, g.defFile.Source)
	if g.defFile.Path != "" {
		destPath = filepath.Join(g.sourceDir, g.defFile.Path)
	}

	dirFiles, err := ioutil.ReadDir(filepath.Dir(srcPath))
	if err != nil {
		return err
	}
	for _, f := range dirFiles {
		match, err := filepath.Match(srcPath, filepath.Join(filepath.Dir(srcPath), f.Name()))
		if err != nil {
			return err
		}
		if match {
			files = append(files, filepath.Join(filepath.Dir(srcPath), f.Name()))
		}
	}

	switch len(files) {
	case 0:
		// Look for the literal file
		_, err = os.Stat(srcPath)
		if err != nil {
			if os.IsNotExist(err) {
				err = fmt.Errorf("File '%s' doesn't exist", srcPath)
			}
			return err
		}
		err = g.doCopy(srcPath, destPath, g.defFile)
	case 1:
		err = g.doCopy(srcPath, destPath, g.defFile)
	default:
		// Make sure that we are copying to a directory
		g.defFile.Path = g.defFile.Path + "/"
		for _, f := range files {
			err = g.doCopy(f, destPath, g.defFile)
			if err != nil {
				break
			}
		}
	}
	return err
}

func (g *copy) doCopy(srcPath, destPath string, defFile shared.DefinitionFile) error {
	in, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	switch in.Mode() & os.ModeType {
	// Regular file
	case 0, os.ModeSymlink:
		if strings.HasSuffix(defFile.Path, "/") {
			destPath = filepath.Join(destPath, filepath.Base(srcPath))
		}
		err := g.copyFile(srcPath, destPath, defFile)
		if err != nil {
			return err
		}

	case os.ModeDir:
		err := g.copyDir(srcPath, destPath, defFile)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("File type of %s not supported", srcPath)
	}

	return nil
}

func (g *copy) copyDir(srcPath, destPath string, defFile shared.DefinitionFile) error {
	err := filepath.Walk(srcPath, func(src string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcPath, src)
		if err != nil {
			return err
		}
		dest := filepath.Join(destPath, rel)
		if err != nil {
			return err
		}

		switch fi.Mode() & os.ModeType {
		case 0, os.ModeSymlink:
			err = g.copyFile(src, dest, defFile)
			if err != nil {
				return err
			}
		case os.ModeDir:
			err := os.MkdirAll(dest, os.ModePerm)
			if err != nil {
				return err
			}
		default:
			fmt.Printf("File type of %s not supported, skipping", src)
		}
		return nil
	})

	return err
}

func (g *copy) copyFile(src, dest string, defFile shared.DefinitionFile) error {
	// Let's make sure that we can create the file
	dir := filepath.Dir(dest)
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
	}
	if err != nil {
		return err
	}

	err = lxd.FileCopy(src, dest)
	if err != nil {
		return err
	}

	out, err := os.Open(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	return updateFileAccess(out, defFile)
}
