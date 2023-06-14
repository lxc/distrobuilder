package sources

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/shared"
)

type vyos struct {
	common
}

func (s *vyos) Run() error {
	isoURL := "https://s3-us.vyos.io/rolling/current/vyos-rolling-latest.iso"

	fpath, err := s.DownloadHash(s.definition.Image, isoURL, "", nil)
	if err != nil {
		return fmt.Errorf("Failed downloading ISO: %w", err)
	}

	err = s.unpackISO(filepath.Join(fpath, "vyos-rolling-latest.iso"), s.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed unpacking ISO: %w", err)
	}

	return nil
}

func (s *vyos) unpackISO(filePath string, rootfsDir string) error {
	isoDir, err := os.MkdirTemp(s.cacheDir, "temp_")
	if err != nil {
		return fmt.Errorf("Failed creating temporary directory: %w", err)
	}

	defer os.RemoveAll(isoDir)

	squashfsDir, err := os.MkdirTemp(s.cacheDir, "temp_")
	if err != nil {
		return fmt.Errorf("Failed creating temporary directory: %w", err)
	}

	defer os.RemoveAll(squashfsDir)

	// this is easier than doing the whole loop thing ourselves
	err = shared.RunCommand(s.ctx, nil, nil, "mount", "-t", "iso9660", "-o", "ro", filePath, isoDir)
	if err != nil {
		return fmt.Errorf("Failed mounting %q: %w", filePath, err)
	}

	defer func() {
		_ = unix.Unmount(isoDir, 0)
	}()

	squashfsImage := filepath.Join(isoDir, "live", "filesystem.squashfs")

	// The squashfs.img contains an image containing the rootfs, so first
	// mount squashfs.img
	err = shared.RunCommand(s.ctx, nil, nil, "mount", "-t", "squashfs", "-o", "ro", squashfsImage, squashfsDir)
	if err != nil {
		return fmt.Errorf("Failed mounting %q: %w", squashfsImage, err)
	}

	defer func() {
		_ = unix.Unmount(squashfsDir, 0)
	}()

	// Remove rootfsDir otherwise rsync will copy the content into the directory
	// itself
	err = os.RemoveAll(rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed removing directory %q: %w", rootfsDir, err)
	}

	s.logger.WithField("file", squashfsImage).Info("Unpacking root image")

	// Since rootfs is read-only, we need to copy it to a temporary rootfs
	// directory in order to create the minimal rootfs.
	err = shared.RsyncLocal(s.ctx, squashfsDir+"/", rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed running rsync: %w", err)
	}

	return nil
}
