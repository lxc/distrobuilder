package generators

import (
	"os"
	p "path"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

// Generator interface.
type Generator interface {
	RunLXC(string, string, *image.LXCImage, shared.DefinitionFile) error
	RunLXD(string, string, *image.LXDImage, shared.DefinitionFile) error
}

// Get returns a Generator.
func Get(generator string) Generator {
	switch generator {
	case "hostname":
		return HostnameGenerator{}
	case "hosts":
		return HostsGenerator{}
	case "remove":
		return RemoveGenerator{}
	case "dump":
		return DumpGenerator{}
	}

	return nil
}

// StoreFile caches a file which can be restored with the RestoreFiles function.
func StoreFile(cacheDir, sourceDir, path string) error {
	// create temporary directory containing old files
	err := os.MkdirAll(filepath.Join(cacheDir, "tmp", p.Dir(path)), 0755)
	if err != nil {
		return err
	}

	return lxd.FileCopy(filepath.Join(sourceDir, path),
		filepath.Join(cacheDir, "tmp", path))
}

// RestoreFiles restores original files which were cached by StoreFile.
func RestoreFiles(cacheDir, sourceDir string) error {
	f := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			// We don't care about directories. They should be present so there's
			// no need to create them.
			return nil
		}

		return lxd.FileCopy(path, filepath.Join(sourceDir,
			strings.TrimPrefix(path, filepath.Join(cacheDir, "tmp"))))
	}

	return filepath.Walk(filepath.Join(cacheDir, "tmp"), f)
}
