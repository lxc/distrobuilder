package sources

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/v3/shared"
)

type openwrt struct {
	common
}

func (s *openwrt) Run() error {
	var baseURL string

	release := s.definition.Image.Release
	releaseInFilename := strings.ToLower(release) + "-"
	// subtargets are subdirectories in url, but not in file names
	architectureInDownloadPath := strings.Replace(s.definition.Image.ArchitectureMapped, "-", "/", 1)

	// Figure out the correct release
	if release == "snapshot" {
		// Build a daily snapshot.
		baseURL = fmt.Sprintf("%s/snapshots/targets/%s/",
			s.definition.Source.URL, architectureInDownloadPath)
		releaseInFilename = ""
	} else {
		baseURL = fmt.Sprintf("%s/releases", s.definition.Source.URL)

		matched, err := regexp.MatchString(`^\d+\.\d+$`, release)
		if err != nil {
			return fmt.Errorf("Failed to match release: %w", err)
		}

		if matched {
			// A release of the form '18.06' has been provided. We need to find
			// out the latest service release of the form '18.06.0'.
			release, err = s.getLatestServiceRelease(baseURL, release)
			if err != nil {
				return fmt.Errorf("Failed to get latest service release: %w", err)
			}

			releaseInFilename = strings.ToLower(release) + "-"
		}

		baseURL = fmt.Sprintf("%s/%s/targets/%s/", baseURL, release, architectureInDownloadPath)
	}

	fname := fmt.Sprintf("openwrt-%s%s-generic-ext4-combined-efi.img.gz", releaseInFilename,
		s.definition.Image.ArchitectureMapped)

	_, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("Failed to parse %q: %w", baseURL, err)
	}

	checksumFile := ""
	if !s.definition.Source.SkipVerification {
		const checksumFileName string = "sha256sums"
		const signatureFileName string = "sha256sums.asc"

		if len(s.definition.Source.Keys) == 0 {
			return errors.New(`GPG keys are required unless "skip_verification: true".`)
		}

		checksumFile = baseURL + checksumFileName
		dirpath, err := s.DownloadHash(s.definition.Image, checksumFile, "", nil)
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", checksumFile, err)
		}

		checksumSignatureFile := baseURL + signatureFileName
		_, err = s.DownloadHash(s.definition.Image, checksumSignatureFile, "", nil)
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", checksumSignatureFile, err)
		}

		valid, err := s.VerifyFile(
			filepath.Join(dirpath, checksumFileName),
			filepath.Join(dirpath, signatureFileName))
		if err != nil {
			return fmt.Errorf(`Failed to verify %q using %q: %w`, checksumFile, checksumSignatureFile, err)
		}

		if !valid {
			return fmt.Errorf(`Invalid signature for %q`, checksumFile)
		}
	}

	fpath, err := s.DownloadHash(s.definition.Image, baseURL+fname, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", baseURL+fname, err)
	}

	s.logger.WithField("file", filepath.Join(fpath, fname)).Info("Unpacking image")

	// gzip issues a warning about trailing garbage (signature) and exits with code 2,
	// which makes go think it failed, while it decompressed just fine.
	// this solution allows gzip to exit with code 0 and 2.
	err = shared.RunScript(s.ctx, fmt.Sprintf(`#!/bin/sh
		gzip -d "%s" || test "$?" -eq 2
		`, filepath.Join(fpath, fname)))
	if err != nil {
		return fmt.Errorf("Failed to decompress combined image: %w", err)
	}

	return s.unpackCombinedImg(filepath.Join(fpath, strings.TrimSuffix(fname, ".gz")), s.rootfsDir)
}

func (s *openwrt) unpackCombinedImg(imagePath, rootfsDir string) error {
	// Image comes with an esp (vfat) and rootfs (ext4) partition
	// We need the bootloader (grub) from the esp as it is not distributed otherwise
	var out strings.Builder
	err := shared.RunCommand(s.ctx, nil, &out, "losetup", "-P", "-f", "--show", imagePath)
	if err != nil {
		return fmt.Errorf("Failed to set up loop device: %w", err)
	}

	loopDevice := strings.TrimSpace(out.String())
	espDevFile := fmt.Sprintf("%sp1", loopDevice)
	rootfsDevFile := fmt.Sprintf("%sp2", loopDevice)

	defer func() { _ = shared.RunCommand(s.ctx, nil, nil, "losetup", "-d", loopDevice) }()

	err = shared.RunCommand(s.ctx, nil, nil, "udevadm", "settle")
	if err != nil {
		return fmt.Errorf("Failed to wait loop device ready: %w", err)
	}

	espTmpDir, err := os.MkdirTemp(s.cacheDir, "temp_")
	if err != nil {
		return fmt.Errorf("Failed to create temporary directory: %w", err)
	}

	defer os.RemoveAll(espTmpDir)

	rootfsTmpDir, err := os.MkdirTemp(s.cacheDir, "temp_")
	if err != nil {
		return fmt.Errorf("Failed to create temporary directory: %w", err)
	}

	defer os.RemoveAll(rootfsTmpDir)

	err = shared.RunCommand(s.ctx, nil, nil, "mount", "-t", "vfat", "-o", "ro", espDevFile, espTmpDir)
	if err != nil {
		return fmt.Errorf("Failed to mount esp from combined image: %w", err)
	}

	defer func() { _ = unix.Unmount(espTmpDir, 0) }()

	err = shared.RunCommand(s.ctx, nil, nil, "mount", "-t", "ext4", "-o", "ro", rootfsDevFile, rootfsTmpDir)
	if err != nil {
		return fmt.Errorf("Failed to mount rootfs from combined image: %w", err)
	}

	defer func() { _ = unix.Unmount(rootfsTmpDir, 0) }()

	// copy over the rootfs
	err = shared.RsyncLocal(s.ctx, rootfsTmpDir+"/", rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed to copy rootfs: %w", err)
	}

	// copy over the contents of the esp to where the esp is mounted on vm's
	// distrobuilder seems to always use "/boot/efi"
	// if we are a container, there is no need for this step, as the boot stuff is
	// useless, but there is no way to distinguish that in the download phase, so
	// we handle it later
	err = os.MkdirAll(filepath.Join(rootfsDir, "boot", "efi"), 0o755)
	if err != nil {
		return fmt.Errorf("Failed to create esp mountpoint: %w", err)
	}

	err = shared.RsyncLocal(s.ctx, espTmpDir+"/", filepath.Join(rootfsDir, "boot", "efi"))
	if err != nil {
		return fmt.Errorf("Failed to copy esp: %w", err)
	}

	return nil
}

func (s *openwrt) getLatestServiceRelease(baseURL, release string) (string, error) {
	var (
		resp *http.Response
		err  error
	)

	err = shared.Retry(func() error {
		resp, err = s.client.Get(baseURL)
		if err != nil {
			return fmt.Errorf("Failed to GET %q: %w", baseURL, err)
		}

		return nil
	}, 3)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to ready body: %w", err)
	}

	regex := regexp.MustCompile(fmt.Sprintf(">(%s\\.\\d+(?:-rc\\d+)?)<", release))
	releases := regex.FindAllStringSubmatch(string(body), -1)

	if len(releases) > 0 {
		return releases[len(releases)-1][1], nil
	}

	return "", errors.New("Failed to find latest service release")
}
