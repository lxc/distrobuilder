package sources

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	incus "github.com/lxc/incus/shared/util"
	"gopkg.in/antchfx/htmlquery.v1"

	"github.com/lxc/distrobuilder/shared"
)

type plamolinux struct {
	common
}

// Run downloads Plamo Linux.
func (s *plamolinux) Run() error {
	releaseStr := strings.TrimSuffix(s.definition.Image.Release, ".x")

	release, err := strconv.Atoi(releaseStr)
	if err != nil {
		return fmt.Errorf("Failed to convert %q: %w", releaseStr, err)
	}

	u, err := url.Parse(s.definition.Source.URL)
	if err != nil {
		return fmt.Errorf("Failed to parse %q: %w", s.definition.Source.URL, err)
	}

	mirrorPath := path.Join(u.Path, fmt.Sprintf("Plamo-%s.x", releaseStr),
		s.definition.Image.ArchitectureMapped, "plamo")

	paths := []string{path.Join(mirrorPath, "00_base")}
	ignoredPkgs := []string{"alsa_utils", "grub", "kernel", "lilo", "linux_firmware", "microcode_ctl",
		"linux_firmwares", "cpufreqd", "cpufrequtils", "gpm", "ntp", "kmod", "kmscon"}

	if release < 7 {
		paths = append(paths, path.Join(mirrorPath, "01_minimum"))
	}

	var pkgDir string

	for _, p := range paths {
		u.Path = p

		pkgDir, err = s.downloadFiles(s.definition.Image, u.String(), ignoredPkgs)
		if err != nil {
			return fmt.Errorf("Failed to download packages: %w", err)
		}
	}

	var pkgTool string

	// Find package tool
	if release < 7 {
		pkgTool = "hdsetup"
	} else {
		pkgTool = "pkgtools8"
	}

	matches, err := filepath.Glob(filepath.Join(pkgDir, fmt.Sprintf("%s-*.t*z*", pkgTool)))
	if err != nil {
		return fmt.Errorf("Failed to match pattern: %w", err)
	}

	if len(matches) == 0 {
		return errors.New("Couldn't find any matching package")
	} else if len(matches) > 1 {
		return errors.New("Found more than one matching package")
	}

	err = shared.RunCommand(s.ctx, nil, nil, "tar", "-pxf", matches[0], "-C", pkgDir, "sbin/")
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", matches[0], err)
	}

	rootfsDirAbs, err := filepath.Abs(s.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed to get absolute path: %w", err)
	}

	err = shared.RunScript(s.ctx, fmt.Sprintf(`#!/bin/sh
set -eux

# Input variables
PKG_DIR="%s"
ROOTFS_DIR="%s"

# Environment
export PATH="${PATH}:${PKG_DIR}/sbin:${PKG_DIR}/sbin/installer"
export LC_ALL="C"
export LANG="C"

# Fix name of installer directory
if [ -d "${PKG_DIR}/sbin/installer_new" ]; then
    [ -d "${PKG_DIR}/sbin/installer" ] && rm -r "${PKG_DIR}/sbin/installer"
    mv "${PKG_DIR}/sbin/installer_new" "${PKG_DIR}/sbin/installer"
fi

# Fix filename of pkgtools8 files
pkg_scripts="installpkg installpkg2 installpkg2.mes makepkg updatepkg removepkg"
for s in $pkg_scripts
do
    if [ -f "${PKG_DIR}/sbin/new_$s" ]; then
        ( cd "${PKG_DIR}/sbin" && mv new_"$s" $s )
    fi
done

# generate symblic link to static-zstd
( cd "${PKG_DIR}/sbin/installer" && ln -sf zstd-* zstd )

# Don't call ldconfig
sed -i "/ldconfig/!s@/sbin@${PKG_DIR}&@g" ${PKG_DIR}/sbin/installpkg*

# Don't override PATH
sed -i "/^export PATH/d" ${PKG_DIR}/sbin/installpkg*

# Install all packages
for pkg in $(ls -cr ${PKG_DIR}/*.t*z*); do
    installpkg -root ${ROOTFS_DIR} -priority ADD ${pkg}
done
`, pkgDir, rootfsDirAbs))
	if err != nil {
		return fmt.Errorf("Failed to run script: %w", err)
	}

	return nil
}

func (s *plamolinux) downloadFiles(def shared.DefinitionImage, URL string, ignoredPkgs []string) (string, error) {
	doc, err := htmlquery.LoadURL(URL)
	if err != nil {
		return "", fmt.Errorf("Failed to load URL %q: %w", URL, err)
	}

	if doc == nil {
		return "", errors.New("Empty HTML document")
	}

	nodes := htmlquery.Find(doc, `//a/@href`)

	var dir string

	for _, n := range nodes {
		target := htmlquery.InnerText(n)

		if strings.HasSuffix(target, ".txz") || strings.HasSuffix(target, ".tzst") {
			pkgName := strings.Split(target, "-")[0]
			if incus.ValueInSlice(pkgName, ignoredPkgs) {
				continue
			}

			// package
			dir, err = s.DownloadHash(def, fmt.Sprintf("%s/%s", URL, target), "", nil)
			if err != nil {
				return "", fmt.Errorf("Failed to download %q: %w", fmt.Sprintf("%s/%s", URL, target), err)
			}
		} else if strings.HasSuffix(target, ".txz/") || strings.HasSuffix(target, ".tzst/") {
			// directory
			u, err := url.Parse(URL)
			if err != nil {
				return "", fmt.Errorf("Failed to parse %q: %w", URL, err)
			}

			u.Path = path.Join(u.Path, target)

			return s.downloadFiles(def, u.String(), ignoredPkgs)
		}
	}

	return dir, nil
}
