package sources

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/antchfx/htmlquery.v1"

	"github.com/lxc/distrobuilder/shared"
)

type slackware struct {
	common
}

// Run downloads Slackware Linux.
func (s *slackware) Run() error {
	u, err := url.Parse(s.definition.Source.URL)
	if err != nil {
		return fmt.Errorf("Failed to parse %q: %w", s.definition.Source.URL, err)
	}

	mirrorPath := ""
	slackpkgPath := ""

	// set mirror path based on architecture
	if s.definition.Image.ArchitectureMapped == "i586" {
		mirrorPath = path.Join(u.Path, fmt.Sprintf("slackware-%s", s.definition.Image.Release), "slackware")
		slackpkgPath = s.definition.Source.URL + fmt.Sprintf("slackware-%s", s.definition.Image.Release)
	} else if s.definition.Image.ArchitectureMapped == "x86_64" {
		mirrorPath = path.Join(u.Path, fmt.Sprintf("slackware64-%s", s.definition.Image.Release), "slackware64")
		slackpkgPath = s.definition.Source.URL + fmt.Sprintf("slackware64-%s", s.definition.Image.Release)
	} else {
		return fmt.Errorf("Invalid architecture: %s", s.definition.Image.Architecture)
	}

	// base software packages and libraries
	paths := []string{path.Join(mirrorPath, "a")}
	// additional required libraries
	paths = append(paths, path.Join(mirrorPath, "ap"), path.Join(mirrorPath, "d"), path.Join(mirrorPath, "l"),
		path.Join(mirrorPath, "n"))
	requiredPkgs := []string{"sysvinit", "sysvinit-scripts", "aaa_base", "aaa_elflibs", "aaa_libraries", "coreutils", "glibc-solibs", "aaa_glibc-solibs", "aaa_terminfo", "pam", "cracklib", "libpwquality", "e2fsprogs", "nvi", "pkgtools", "shadow", "tar", "xz", "bash", "etc", "gzip", "pcre2", "libpsl", "wget", "gnupg", "elvis", "slackpkg", "ncurses", "bin", "bzip2", "grep", "acl", "pcre", "gmp", "attr", "sed", "dialog", "file", "gawk", "time", "gettext", "libcgroup", "patch", "sysfsutils", "time", "tree", "utempter", "which", "util-linux", "elogind", "libseccomp", "mpfr", "libunistring", "diffutils", "procps", "findutils", "iproute2", "dhcpcd", "openssl", "perl", "ca-certificates", "inetd", "iputils", "libmnl", "network-scripts", "libaio", "glibc", "nano", "hostname"}

	var pkgDir string

	for _, p := range paths {
		u.Path = p

		pkgDir, err = s.downloadFiles(s.definition.Image, u.String(), requiredPkgs)
		if err != nil {
			return fmt.Errorf("Failed to download packages: %w", err)
		}
	}

	// find package tools
	matches, err := filepath.Glob(filepath.Join(pkgDir, "pkgtools-*.t*z"))
	if err != nil {
		return fmt.Errorf("Failed to match pattern: %w", err)
	}

	err = shared.RunCommand(s.ctx, nil, nil, "tar", "-pxf", matches[0], "-C", pkgDir, "sbin/")
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", matches[0], err)
	}

	rootfsDirAbs, err := filepath.Abs(s.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed to get absolute path: %w", err)
	}

	// build rootfs
	err = shared.RunScript(s.ctx, fmt.Sprintf(`#!/bin/sh
set -eux

# Input variables
PKG_DIR="%s"
ROOTFS_DIR="%s"

# Environment
export LC_ALL="C"
export LANG="C"

# Don't override PATH
sed -i "/^export PATH/d" ${PKG_DIR}/sbin/installpkg*

# Install all packages
# not compatible with versions < 13.37
for pkg in $(ls -cr ${PKG_DIR}/*.t*z); do
	# Prevent install script for sysvinit from trying to run /sbin/init
	if (echo ${pkg} | grep -E 'sysvinit-[0-9]') ; then
		mkdir -p ${PKG_DIR}/sysvinit && cd ${PKG_DIR}/sysvinit
		tar -xf ${pkg}
		sed -i 's@/sbin/init@#/sbin/init@' install/doinst.sh
		tar -cJf ${pkg} *
		${PKG_DIR}/sbin/installpkg --terse --root ${ROOTFS_DIR} ${pkg}
		cd -
	else
    	${PKG_DIR}/sbin/installpkg --terse --root ${ROOTFS_DIR} ${pkg}
	fi
done

# Disable kernel/sys modifications
sed -i 's@/bin/dmesg@#/bin/dmesg@g' ${ROOTFS_DIR}/etc/rc.d/rc.M
sed -i 's@/sbin/modprobe@echo@g' ${ROOTFS_DIR}/etc/rc.d/rc.inet1
if [ -f ${ROOTFS_DIR}/etc/rc.d/rc.elogind ]; then
	sed -i 's@cd /sys/fs/cgroup;@@g' ${ROOTFS_DIR}/etc/rc.d/rc.elogind
fi

# Enable networking on eth0
sed -i 's/USE_DHCP\[0\]=""/USE_DHCP\[0\]="yes"/' ${ROOTFS_DIR}/etc/rc.d/rc.inet1.conf
sed -i 's/USE_DHCP6\[0\]=""/USE_DHCP6\[0\]="yes"/' ${ROOTFS_DIR}/etc/rc.d/rc.inet1.conf

# Some services expect fstab
touch ${ROOTFS_DIR}/etc/fstab

# Add mirror to slackpkg
mkdir -p ${ROOTFS_DIR}/etc/slackpkg
echo "%s" > ${ROOTFS_DIR}/etc/slackpkg/mirrors
`, pkgDir, rootfsDirAbs, slackpkgPath))
	if err != nil {
		return fmt.Errorf("Failed to run script: %w", err)
	}

	return nil
}

func (s *slackware) downloadFiles(def shared.DefinitionImage, URL string, requiredPkgs []string) (string, error) {
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

		if strings.HasSuffix(target, ".txz") || strings.HasSuffix(target, ".tgz") {
			pkgName := strings.Split(target, "-")[0]
			twoPkgName := strings.Split(target, "-")[0] + "-" + strings.Split(target, "-")[1]

			if !((slices.Contains(requiredPkgs, pkgName)) || (slices.Contains(requiredPkgs, twoPkgName))) {
				continue
			}

			// package
			dir, err = s.DownloadHash(def, fmt.Sprintf("%s/%s", URL, target), "", nil)
			if err != nil {
				return "", fmt.Errorf("Failed to download %q: %w", fmt.Sprintf("%s/%s", URL, target), err)
			}
		} else if strings.HasSuffix(target, ".txz/") || strings.HasSuffix(target, ".tgz/") {
			// directory
			u, err := url.Parse(URL)
			if err != nil {
				return "", fmt.Errorf("Failed to parse %q: %w", URL, err)
			}

			u.Path = path.Join(u.Path, target)

			return s.downloadFiles(def, u.String(), requiredPkgs)
		}
	}

	return dir, nil
}
