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
	"strconv"
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
	err := shared.RunCommand(s.ctx, nil, &out, "fdisk", "-l", "-o", "Start,Sectors", imagePath)
	if err != nil {
		return fmt.Errorf(`failed to run "fdisk": %w`, err)
	}

	// we expect the first match to be the esp, the second to be the rootfs
	// openwrt also has a very small third partition (number 128) for some legacy stuff
	// should match the first two rows from a table like this
	//
	// Start Sectors
	//   512   32768
	// 33280  212992
	//    34     478
	regex := regexp.MustCompile(`\n\s*(\d+)\s+(\d+)`)
	matches := regex.FindAllStringSubmatch(out.String(), 2)
	if len(matches) != 2 {
		return fmt.Errorf(`Failed to parse output of "fdisk"; unexpected number of matches: %d`, len(matches))
	}

	// a partition has an offset and size in bytes; we are looking for 2 partitions:
	// - first one is the esp
	// - the second one is the rootfs
	var partitions [2][2]int

	for i := 0; i <= 1; i++ {
		// offset
		partitions[i][0], err = strconv.Atoi(matches[i][1])
		if err != nil {
			return fmt.Errorf("Failed to parse partition offset: %w", err)
		}

		// size
		partitions[i][1], err = strconv.Atoi(matches[i][2])
		if err != nil {
			return fmt.Errorf("Failed to parse partition size: %w", err)
		}

		// openwrt uses a sector size of 512; could detect this, but not likely to change
		// fdisk reports in sectors, mount needs the numbers in bytes
		partitions[i][0] *= 512
		partitions[i][1] *= 512
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

	// we need the sizelimit= option in the mount command to be able to mount _both_ of the partitions
	// at the same time
	err = shared.RunCommand(s.ctx, nil, nil, "mount", "-t", "vfat",
		"-o", fmt.Sprintf("ro,loop,offset=%d,sizelimit=%d", partitions[0][0], partitions[0][1]),
		imagePath, espTmpDir)
	if err != nil {
		return fmt.Errorf("Failed to mount esp from combined image: %w", err)
	}

	defer func() { _ = unix.Unmount(espTmpDir, 0) }()

	err = shared.RunCommand(s.ctx, nil, nil, "mount", "-t", "ext4",
		"-o", fmt.Sprintf("ro,loop,offset=%d,sizelimit=%d", partitions[1][0], partitions[1][1]),
		imagePath, rootfsTmpDir)
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

	if len(releases) == 0 {
		return "", errors.New("Failed to find latest service release")
	}

	// release candidates only exist for the first stable release of the given
	// major, and the lexicographic sorting used by the download page always
	// lists them after the first stable. Hence we check for a possible stable
	// in the first position when the last entry is an rc release.
	latest := releases[len(releases)-1][1]
	first := releases[0][1]
	if strings.Contains(latest, "-rc") && !strings.Contains(first, "-rc") {
		latest = first
	}

	return latest, nil
}
