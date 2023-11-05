package sources

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-github/v56/github"
	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/shared"
)

type vyos struct {
	common

	fname string
	fpath string
}

func (s *vyos) Run() error {
	err := s.downloadImage(s.definition)
	if err != nil {
		return fmt.Errorf("Failed to download image: %w", err)
	}

	return s.unpackISO(filepath.Join(s.fpath, s.fname), s.rootfsDir)
}

func (s *vyos) downloadImage(definition shared.Definition) error {
	var err error

	ctx := context.Background()
	client := github.NewClient(nil)
	owner := "vyos"
	repo := "vyos-rolling-nightly-builds"

	latestRelease, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("Failed to get latest release, %w", err)
	}

	isoURL := ""
	assets := latestRelease.Assets
	for _, a := range assets {
		ext := filepath.Ext(a.GetName())
		if ext == ".iso" {
			isoURL = a.GetBrowserDownloadURL()
			s.fname = a.GetName()
		}
	}

	if isoURL == "" {
		return fmt.Errorf("Failed to get latest release URL.")
	}

	s.fpath, err = s.DownloadHash(s.definition.Image, isoURL, "", nil)

	return err
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
