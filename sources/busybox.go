package sources

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

type busybox struct {
	common
}

// Run downloads a busybox tarball.
func (s *busybox) Run() error {
	fname := fmt.Sprintf("busybox-%s.tar.bz2", s.definition.Image.Release)
	tarball := fmt.Sprintf("%s/%s", s.definition.Source.URL, fname)

	var (
		fpath string
		err   error
	)

	if s.definition.Source.SkipVerification {
		fpath, err = s.DownloadHash(s.definition.Image, tarball, "", nil)

	} else {
		fpath, err = s.DownloadHash(s.definition.Image, tarball, tarball+".sha256", sha256.New())
	}
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", tarball, err)
	}

	sourceDir := filepath.Join(s.cacheDir, "src")

	err = os.MkdirAll(sourceDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", sourceDir, err)
	}

	s.logger.Infow("Unpacking image", "file", filepath.Join(fpath, fname))

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), sourceDir, false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", fname, err)
	}

	sourceDir = filepath.Join(sourceDir, fmt.Sprintf("busybox-%s", s.definition.Image.Release))

	err = shared.RunScript(s.ctx, fmt.Sprintf(`#!/bin/sh
set -eux

source_dir=%s
rootfs_dir=%s

cwd="$(pwd)"

cd "${source_dir}"
make defconfig
sed -ri 's/# CONFIG_STATIC .*/CONFIG_STATIC=y/g' .config
make

cd "${cwd}"
mkdir -p "${rootfs_dir}/bin"
mv ${source_dir}/busybox "${rootfs_dir}/bin/busybox"
`, sourceDir, s.rootfsDir))
	if err != nil {
		return fmt.Errorf("Failed to build busybox: %w", err)
	}

	var buf bytes.Buffer

	err = lxd.RunCommandWithFds(os.Stdin, &buf, filepath.Join(s.rootfsDir, "bin", "busybox"), "--list-full")
	if err != nil {
		return fmt.Errorf("Failed to install busybox: %w", err)
	}

	scanner := bufio.NewScanner(&buf)

	for scanner.Scan() {
		path := filepath.Join(s.rootfsDir, scanner.Text())

		if path == "" || path == "bin/busybox" {
			continue
		}

		s.logger.Debugf("Creating directory %q", path)

		err = os.MkdirAll(filepath.Dir(path), 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", filepath.Dir(path), err)
		}

		s.logger.Debugf("Creating symlink %q -> %q", path, "/bin/busybox")

		err = os.Symlink("/bin/busybox", path)
		if err != nil {
			return fmt.Errorf("Failed to create symlink %q -> /bin/busybox: %w", path, err)
		}
	}

	for _, path := range []string{"dev", "mnt", "proc", "root", "sys", "tmp"} {
		err := os.Mkdir(filepath.Join(s.rootfsDir, path), 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", filepath.Join(s.rootfsDir, path), err)
		}
	}

	return nil
}
