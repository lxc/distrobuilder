package sources

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	lxd "github.com/lxc/lxd/shared"
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
	releaseStr := strings.TrimSuffix(definition.Image.Release, ".x")

	release, err := strconv.Atoi(releaseStr)
	if err != nil {
		return fmt.Errorf("Failed to determine release: %v", err)
	}

	u, err := url.Parse(definition.Source.URL)
	if err != nil {
		return err
	}

	mirrorPath := path.Join(u.Path, fmt.Sprintf("Plamo-%s.x", releaseStr),
		definition.Image.ArchitectureMapped, "plamo")

	paths := []string{path.Join(mirrorPath, "00_base")}
	ignoredPkgs := []string{"alsa_utils", "grub", "kernel", "lilo", "linux_firmware", "microcode_ctl",
		"linux_firmwares", "cpufreqd", "cpufrequtils", "gpm", "ntp", "kmod", "kmscon"}

	if release < 7 {
		paths = append(paths, path.Join(mirrorPath, "01_minimum"))
	}

	var pkgDir string

	for _, p := range paths {
		u.Path = p

		pkgDir, err = s.downloadFiles(definition.Image, u.String(), ignoredPkgs)
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

	rootfsDirAbs, err := filepath.Abs(rootfsDir)
	if err != nil {
		return err
	}

	return shared.RunScript(fmt.Sprintf(`#!/bin/sh
set -eux

# Input variables
PKG_DIR="%s"
ROOTFS_DIR="%s"

# Environment
export PATH="${PKG_DIR}/sbin:${PATH}"
export LC_ALL="C"
export LANG="C"

# Don't call ldconfig
sed -i "/ldconfig/!s@/sbin@${PKG_DIR}&@g" ${PKG_DIR}/sbin/installpkg*

# Install all packages
for pkg in $(ls -cr ${PKG_DIR}/*.t?z); do
    installpkg -root ${ROOTFS_DIR} -priority ADD ${pkg}
done
`, pkgDir, rootfsDirAbs))
}

func (s *PlamoLinuxHTTP) downloadFiles(def shared.DefinitionImage, URL string, ignoredPkgs []string) (string, error) {
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
			pkgName := strings.Split(target, "-")[0]
			if lxd.StringInSlice(pkgName, ignoredPkgs) {
				continue
			}

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

			return s.downloadFiles(def, u.String(), ignoredPkgs)
		}
	}

	return dir, nil
}
