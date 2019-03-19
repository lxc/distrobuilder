package sources

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/antchfx/htmlquery.v1"

	"github.com/lxc/distrobuilder/shared"
)

// PlamoLinuxHTTP represents the Plamo Linux downloader.
type PlamoLinuxHTTP struct {
}

// NewPlamoLinuxHTTP creates a new PlamoLinuxHTTP instance.
func NewPlamoLinuxHTTP() *PlamoLinuxHTTP {
	return &PlamoLinuxHTTP{}
}

// Run downloads Plamo Linux.
func (s *PlamoLinuxHTTP) Run(definition shared.Definition, rootfsDir string) error {
	release, err := strconv.Atoi(definition.Image.Release)
	if err != nil {
		return fmt.Errorf("Failed to determine release: %v", err)
	}

	u, err := url.Parse(definition.Source.URL)
	if err != nil {
		return err
	}

	mirrorPath := path.Join(u.Path, fmt.Sprintf("Plamo-%s.x", definition.Image.Release),
		definition.Image.ArchitectureMapped, "plamo")

	paths := []string{
		path.Join(mirrorPath, "00_base"),
	}

	if release < 7 {
		paths = append(paths, path.Join(mirrorPath, "01_minimum"))

	}

	var pkgDir string

	for _, p := range paths {
		u.Path = p

		pkgDir, err = s.downloadFiles(definition.Image, u.String())
		if err != nil {
			return fmt.Errorf("Failed to download packages: %v", err)
		}
	}

	var pkgTool string

	// Find package tool
	if release < 7 {
		pkgTool = "hdsetup"
	} else {
		pkgTool = "pkgtools"
	}

	matches, err := filepath.Glob(filepath.Join(pkgDir, fmt.Sprintf("%s-*.txz", pkgTool)))
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		return fmt.Errorf("Couldn't find any matching package")
	} else if len(matches) > 1 {
		return fmt.Errorf("Found more than one matching package")
	}

	err = shared.RunCommand("tar", "-pxJf", matches[0], "-C", pkgDir, "sbin/")
	if err != nil {
		return err
	}

	return shared.RunScript(fmt.Sprintf(`#!/bin/sh

dlcache=%s
rootfs_dir=%s

sed -i "/ldconfig/!s@/sbin@$dlcache&@g" $dlcache/sbin/installpkg*
PATH=$dlcache/sbin:$PATH
export LC_ALL=C LANG=C

for p in $(ls -cr $dlcache/*.t?z) ; do
    installpkg -root $rootfs_dir -priority ADD $p
done
`, pkgDir, rootfsDir))
}

func (s *PlamoLinuxHTTP) downloadFiles(def shared.DefinitionImage, URL string) (string, error) {
	doc, err := htmlquery.LoadURL(URL)
	if err != nil {
		return "", err
	}

	if doc == nil {
		return "", fmt.Errorf("Failed to load URL")
	}

	nodes := htmlquery.Find(doc, `//a/@href`)

	var dir string

	for _, n := range nodes {
		target := htmlquery.InnerText(n)

		if strings.HasSuffix(target, ".txz") {
			// package
			dir, err = shared.DownloadHash(def, fmt.Sprintf("%s/%s", URL, target), "", nil)
			if err != nil {
				return "", err
			}
		} else if strings.HasSuffix(target, ".txz/") {
			// directory
			u, err := url.Parse(URL)
			if err != nil {
				return "", err
			}

			u.Path = path.Join(u.Path, target)

			return s.downloadFiles(def, u.String())
		}
	}

	return dir, nil
}
