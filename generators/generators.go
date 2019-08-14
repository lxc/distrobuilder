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
	case "cloud-init":
		return CloudInitGenerator{}
	}

	return nil
}

var storedFiles = map[string]os.FileInfo{}

// StoreFile caches a file which can be restored with the RestoreFiles function.
func StoreFile(cacheDir, sourceDir, path string) error {
	fullPath := filepath.Join(sourceDir, path)

	_, ok := storedFiles[fullPath]
	if ok {
		// This file or directory has already been recorded
		return nil
	}

	// Record newly created files
	if !lxd.PathExists(fullPath) {
		storedFiles[fullPath] = nil
		return nil
	}

	// create temporary directory containing old files
	err := os.MkdirAll(filepath.Join(cacheDir, "tmp", p.Dir(path)), 0755)
	if err != nil {
		return err
	}

	info, err := os.Lstat(fullPath)
	if err != nil {
		return err
	}

	storedFiles[fullPath] = info

	err = os.Rename(fullPath, filepath.Join(cacheDir, "tmp", path))
	if err == nil {
		return nil
	}

	// Try copying the file since renaming it failed
	if info.IsDir() {
		err = lxd.DirCopy(fullPath, filepath.Join(cacheDir, "tmp", path))
	} else {
		err = lxd.FileCopy(fullPath, filepath.Join(cacheDir, "tmp", path))
	}
	if err != nil {
		return err
	}

	return os.RemoveAll(fullPath)
}

// RestoreFiles restores original files which were cached by StoreFile.
func RestoreFiles(cacheDir, sourceDir string) error {
	var err error

	for origPath, fi := range storedFiles {
		// Deal with newly created files
		if fi == nil {
			err := os.RemoveAll(origPath)
			if err != nil {
				return err
			}

			continue
		}

		err = os.Rename(filepath.Join(cacheDir, "tmp", strings.TrimPrefix(origPath, sourceDir)), origPath)
		if err == nil {
			continue
		}

		// Try copying the file or directory since renaming it failed
		if fi.IsDir() {
			err = lxd.DirCopy(filepath.Join(cacheDir, "tmp", strings.TrimPrefix(origPath, sourceDir)), origPath)
		} else {
			err = lxd.FileCopy(filepath.Join(cacheDir, "tmp", strings.TrimPrefix(origPath, sourceDir)), origPath)
		}
		if err != nil {
			return err
		}
	}

	// Reset the list of stored files
	storedFiles = map[string]os.FileInfo{}

	return nil
}
