package generators

import (
	"os"
	p "path"
	"path/filepath"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

// Generator interface.
type Generator interface {
	RunLXC(string, string, *image.LXCImage, shared.DefinitionFile) error
	RunLXD(string, string, *image.LXDImage, shared.DefinitionFile) error
	Run(string, string, shared.DefinitionFile) error
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
	case "template":
		return TemplateGenerator{}
	case "upstart-tty":
		return UpstartTTYGenerator{}
	}

	return nil
}

var storedFiles = map[string]string{}

// StoreFile caches a file which can be restored with the RestoreFiles function.
func StoreFile(cacheDir, sourceDir, path string) error {
	// Record newly created files
	if !lxd.PathExists(filepath.Join(sourceDir, path)) {
		storedFiles[filepath.Join(sourceDir, path)] = ""
		return nil
	}

	// create temporary directory containing old files
	err := os.MkdirAll(filepath.Join(cacheDir, "tmp", p.Dir(path)), 0755)
	if err != nil {
		return err
	}

	storedFiles[filepath.Join(sourceDir, path)] = filepath.Join(cacheDir, "tmp", path)

	return lxd.FileCopy(filepath.Join(sourceDir, path),
		filepath.Join(cacheDir, "tmp", path))
}

// RestoreFiles restores original files which were cached by StoreFile.
func RestoreFiles(cacheDir, sourceDir string) error {
	for origPath, tmpPath := range storedFiles {
		// Deal with newly created files
		if tmpPath == "" {
			err := os.Remove(origPath)
			if err != nil {
				return err
			}

			continue
		}

		err := lxd.FileCopy(tmpPath, origPath)
		if err != nil {
			return err
		}
	}

	// Reset the list of stored files
	storedFiles = map[string]string{}

	return nil
}
