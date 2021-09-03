package sources

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

type openwrt struct {
	common
}

// Run downloads the tarball and unpacks it.
func (s *openwrt) Run() error {
	var baseURL string

	release := s.definition.Image.Release
	releaseInFilename := strings.ToLower(release) + "-"

	var architecturePath string

	switch s.definition.Image.ArchitectureMapped {
	case "x86_64":
		architecturePath = strings.Replace(s.definition.Image.ArchitectureMapped, "_", "/", 1)
	case "armv7l":
		architecturePath = "armvirt/32"
	case "aarch64":
		architecturePath = "armvirt/64"
	}

	// Figure out the correct release
	if release == "snapshot" {
		// Build a daily snapshot.
		baseURL = fmt.Sprintf("%s/snapshots/targets/%s/",
			s.definition.Source.URL, architecturePath)
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

		baseURL = fmt.Sprintf("%s/%s/targets/%s/", baseURL, release, architecturePath)
	}

	var fname string

	if release == "snapshot" {
		switch s.definition.Image.ArchitectureMapped {
		case "x86_64":
			fname = fmt.Sprintf("openwrt-%s%s-rootfs.tar.gz", releaseInFilename,
				strings.Replace(architecturePath, "/", "-", 1))
		case "armv7l":
			fallthrough
		case "aarch64":
			fname = fmt.Sprintf("openwrt-%s-default-rootfs.tar.gz",
				strings.Replace(architecturePath, "/", "-", 1))
		}

	} else {
		switch s.definition.Image.ArchitectureMapped {
		case "x86_64":
			if strings.HasPrefix(release, "21.02") {
				fname = fmt.Sprintf("openwrt-%s%s-rootfs.tar.gz", releaseInFilename,
					strings.Replace(architecturePath, "/", "-", 1))
			} else {
				fname = fmt.Sprintf("openwrt-%s%s-generic-rootfs.tar.gz", releaseInFilename,
					strings.Replace(architecturePath, "/", "-", 1))
			}
		case "armv7l":
			fallthrough
		case "aarch64":
			fname = fmt.Sprintf("openwrt-%s%s-default-rootfs.tar.gz", releaseInFilename,
				strings.Replace(architecturePath, "/", "-", 1))
		}
	}

	resp, err := http.Head(baseURL)
	if err != nil {
		return fmt.Errorf("Failed to HEAD %q: %w", baseURL, err)
	}

	// Use fallback image "generic"
	if resp.StatusCode == http.StatusNotFound && s.definition.Image.ArchitectureMapped == "x86_64" {
		baseURL = strings.ReplaceAll(baseURL, "x86/64", "x86/generic")
		baseURL = strings.ReplaceAll(baseURL, "x86-64", "x86-generic")
		fname = strings.ReplaceAll(fname, "x86-64", "x86-generic")
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("Failed to parse %q: %w", baseURL, err)
	}

	checksumFile := ""
	if !s.definition.Source.SkipVerification {
		if len(s.definition.Source.Keys) != 0 {
			checksumFile = baseURL + "sha256sums"
			_, err := shared.DownloadHash(s.definition.Image, checksumFile, "", nil)
			if err != nil {
				return fmt.Errorf("Failed to download %q: %w", checksumFile, err)
			}
		} else {
			// Force gpg checks when using http
			if url.Scheme != "https" {
				return errors.New("GPG keys are required if downloading from HTTP")
			}
		}
	}

	fpath, err := shared.DownloadHash(s.definition.Image, baseURL+fname, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", baseURL+fname, err)
	}

	sdk, err := s.getSDK(baseURL, release)
	if err != nil {
		return fmt.Errorf("Failed to get SDK: %w", err)
	}

	_, err = shared.DownloadHash(s.definition.Image, baseURL+sdk, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", baseURL+sdk, err)
	}

	lxdOpenWrtTarball := "https://github.com/mikma/lxd-openwrt/archive/master.tar.gz"

	_, err = shared.DownloadHash(s.definition.Image, lxdOpenWrtTarball, "", sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", lxdOpenWrtTarball, err)
	}

	tempScriptsDir := filepath.Join(s.cacheDir, "fixes", "lxd-openwrt-master")
	tempSDKDir := filepath.Join(tempScriptsDir, "build_dir")

	err = os.MkdirAll(tempSDKDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", tempSDKDir, err)
	}

	err = os.MkdirAll(tempScriptsDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", tempScriptsDir, err)
	}

	s.logger.Infow("Unpacking image", "file", filepath.Join(fpath, fname))

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", filepath.Join(fpath, fname), err)
	}

	s.logger.Infow("Unpacking repository tarball", "file", filepath.Join(fpath, "master.tar.gz"))

	err = lxd.Unpack(filepath.Join(fpath, "master.tar.gz"), filepath.Join(s.cacheDir, "fixes"), false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", filepath.Join(fpath, "master.tar.gz"), err)
	}

	s.logger.Infow("Unpacking sdk", "file", filepath.Join(fpath, sdk))

	err = lxd.Unpack(filepath.Join(fpath, sdk), tempSDKDir, false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", filepath.Join(fpath, sdk), err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Failed to get current working directory: %w", err)
	}
	defer os.Chdir(currentDir)

	// Set environment used in the lxd-openwrt scripts
	os.Setenv("OPENWRT_ROOTFS", filepath.Join(fpath, fname))

	// Always use an absolute path
	if strings.HasPrefix(s.rootfsDir, "/") {
		os.Setenv("OPENWRT_ROOTFS_DIR", s.rootfsDir)
	} else {
		os.Setenv("OPENWRT_ROOTFS_DIR", filepath.Join(currentDir, s.rootfsDir))
	}

	os.Setenv("OPENWRT_SDK", fmt.Sprintf("build_dir/%s", strings.TrimSuffix(sdk, ".tar.xz")))

	if s.definition.Image.Architecture == "armv7l" {
		os.Setenv("OPENWRT_ARCH", "aarch32")

	} else {
		os.Setenv("OPENWRT_ARCH", s.definition.Image.Architecture)
	}

	os.Setenv("OPENWRT_VERSION", release)

	err = os.Chdir(tempScriptsDir)
	if err != nil {
		return fmt.Errorf("Failed to change the current working directory to %q: %w", tempScriptsDir, err)
	}

	f, err := os.Open("build.sh")
	if err != nil {
		return fmt.Errorf(`Failed to open "build.sh": %w`, err)
	}

	var newContent strings.Builder
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "arch_lxd=") {
			newContent.WriteString("arch_lxd=${OPENWRT_ARCH}\n")
			continue
		}

		if strings.HasPrefix(scanner.Text(), "ver=") {
			newContent.WriteString("ver=${OPENWRT_VERSION}\nreadonly rootfs=${OPENWRT_ROOTFS}\nreadonly sdk=${OPENWRT_SDK}\n")
			continue
		}

		if scanner.Text() == "download_rootfs" {
			continue
		}

		if scanner.Text() == "download_sdk" {
			continue
		}

		newContent.WriteString(scanner.Text() + "\n")
	}

	f.Close()

	err = ioutil.WriteFile("build.sh", []byte(newContent.String()), 0755)
	if err != nil {
		return fmt.Errorf(`Failed to write to build.sh": %w`, err)
	}

	f, err = os.Open("scripts/build_rootfs.sh")
	if err != nil {
		return fmt.Errorf(`Failed to open "scripts/build_rootfs.sh": %w`, err)
	}

	newContent.Reset()

	scanner = bufio.NewScanner(f)

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "dir=") {
			newContent.WriteString(fmt.Sprintf("dir=%s\n", s.cacheDir))
			continue
		}

		if strings.HasPrefix(scanner.Text(), "instroot=") {
			newContent.WriteString("instroot=${OPENWRT_ROOTFS_DIR}\n")
			continue
		}

		if scanner.Text() == "unpack" {
			continue
		}

		if scanner.Text() == "pack" {
			continue
		}

		newContent.WriteString(scanner.Text() + "\n")
	}

	f.Close()

	err = ioutil.WriteFile("scripts/build_rootfs.sh", []byte(newContent.String()), 0755)
	if err != nil {
		return fmt.Errorf(`Failed to write to "scripts/build_rootfs.sh": %w`, err)
	}

	_, err = lxd.RunCommand("sh", "build.sh")
	if err != nil {
		return fmt.Errorf(`Failed to run "build.sh": %w`, err)
	}

	return nil
}

func (s *openwrt) getLatestServiceRelease(baseURL, release string) (string, error) {
	resp, err := http.Get(baseURL)
	if err != nil {
		return "", fmt.Errorf("Failed to GET %q: %w", baseURL, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to ready body: %w", err)
	}

	regex := regexp.MustCompile(fmt.Sprintf(">(%s\\.\\d+)<", release))
	releases := regex.FindAllStringSubmatch(string(body), -1)

	if len(releases) > 0 {
		return releases[len(releases)-1][1], nil
	}

	return "", errors.New("Failed to find latest service release")
}

func (s *openwrt) getSDK(baseURL, release string) (string, error) {
	resp, err := http.Get(baseURL)
	if err != nil {
		return "", fmt.Errorf("Failed to GET %q: %w", baseURL, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to read body: %w", err)
	}

	if release == "snapshot" {
		release = ""
	} else {
		release = fmt.Sprintf("-%s", release)
	}

	regex := regexp.MustCompile(fmt.Sprintf(">(openwrt-sdk%s-.*\\.tar\\.xz)<", release))
	releases := regex.FindAllStringSubmatch(string(body), -1)

	if len(releases) > 0 {
		return releases[len(releases)-1][1], nil
	}

	return "", errors.New("Failed to find SDK")
}
