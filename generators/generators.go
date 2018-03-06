package generators

import (
	"os"
	p "path"
	"path/filepath"
	"strings"

	"github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/image"
)

// Generator interface.
type Generator interface {
	CreateLXCData(string, string, *image.LXCImage) error
	CreateLXDData(string, string, *image.LXDImage) error
}

// Get returns a Generator.
func Get(generator string) Generator {
	switch generator {
	case "hostname":
		return HostnameGenerator{}
	case "hosts":
		return HostsGenerator{}
	}

	return nil
}

// StoreFile caches a file which can be restored with the RestoreFiles function.
func StoreFile(cacheDir, path string) error {
	// create temporary directory containing old files
	err := os.MkdirAll(filepath.Join(cacheDir, "tmp", p.Dir(path)), 0755)
	if err != nil {
		return err
	}

	return shared.FileCopy(filepath.Join(cacheDir, "rootfs", path),
		filepath.Join(cacheDir, "tmp", path))
}

// RestoreFiles restores original files which were cached by StoreFile.
func RestoreFiles(cacheDir string) error {
	f := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			// We don't care about directories. They should be present so there's
			// no need to create them.
			return nil
		}

		return os.Rename(path, filepath.Join(cacheDir, "rootfs",
			strings.TrimPrefix(path, filepath.Join(cacheDir, "tmp"))))
	}

	return filepath.Walk(filepath.Join(cacheDir, "tmp"), f)
}
