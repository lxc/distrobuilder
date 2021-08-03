package sources

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"

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
			return errors.Wrap(err, "Failed to match release")
		}

		if matched {
			// A release of the form '18.06' has been provided. We need to find
			// out the latest service release of the form '18.06.0'.
			release, err = s.getLatestServiceRelease(baseURL, release)
			if err != nil {
				return errors.Wrap(err, "Failed to get latest service release")
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
		return errors.Wrapf(err, "Failed to HEAD %q", baseURL)
	}

	// Use fallback image "generic"
	if resp.StatusCode == http.StatusNotFound && s.definition.Image.ArchitectureMapped == "x86_64" {
		baseURL = strings.ReplaceAll(baseURL, "x86/64", "x86/generic")
		baseURL = strings.ReplaceAll(baseURL, "x86-64", "x86-generic")
		fname = strings.ReplaceAll(fname, "x86-64", "x86-generic")
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse %q", baseURL)
	}

	checksumFile := ""
	if !s.definition.Source.SkipVerification {
		if len(s.definition.Source.Keys) != 0 {
			checksumFile = baseURL + "sha256sums"
			_, err := shared.DownloadHash(s.definition.Image, checksumFile, "", nil)
			if err != nil {
				return errors.Wrapf(err, "Failed to download %q", checksumFile)
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
		return errors.Wrapf(err, "Failed to download %q", baseURL+fname)
	}

	sdk, err := s.getSDK(baseURL, release)
	if err != nil {
		return errors.Wrap(err, "Failed to get SDK")
	}

	_, err = shared.DownloadHash(s.definition.Image, baseURL+sdk, checksumFile, sha256.New())
	if err != nil {
		return errors.Wrapf(err, "Failed to download %q", baseURL+sdk)
	}

	lxdOpenWrtTarball := "https://github.com/mikma/lxd-openwrt/archive/master.tar.gz"

	_, err = shared.DownloadHash(s.definition.Image, lxdOpenWrtTarball, "", sha256.New())
	if err != nil {
		return errors.Wrapf(err, "Failed to download %q", lxdOpenWrtTarball)
	}

	tempScriptsDir := filepath.Join(s.cacheDir, "fixes", "lxd-openwrt-master")
	tempSDKDir := filepath.Join(tempScriptsDir, "build_dir")

	err = os.MkdirAll(tempSDKDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "Failed to create directory %q", tempSDKDir)
	}

	err = os.MkdirAll(tempScriptsDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "Failed to create directory %q", tempScriptsDir)
	}

	s.logger.Infow("Unpacking image", "file", filepath.Join(fpath, fname))

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to unpack %q", filepath.Join(fpath, fname))
	}

	s.logger.Infow("Unpacking repository tarball", "file", filepath.Join(fpath, "master.tar.gz"))

	err = lxd.Unpack(filepath.Join(fpath, "master.tar.gz"), filepath.Join(s.cacheDir, "fixes"), false, false, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to unpack %q", filepath.Join(fpath, "master.tar.gz"))
	}

	s.logger.Infow("Unpacking sdk", "file", filepath.Join(fpath, sdk))

	err = lxd.Unpack(filepath.Join(fpath, sdk), tempSDKDir, false, false, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to unpack %q", filepath.Join(fpath, sdk))
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "Failed to get current working directory")
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
		return errors.Wrapf(err, "Failed to change the current working directory to %q", tempScriptsDir)
	}

	f, err := os.Open("build.sh")
	if err != nil {
		return errors.Wrap(err, `Failed to open "build.sh"`)
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
		return errors.Wrap(err, `Failed to write to build.sh"`)
	}

	f, err = os.Open("scripts/build_rootfs.sh")
	if err != nil {
		return errors.Wrap(err, `Failed to open "scripts/build_rootfs.sh"`)
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
		return errors.Wrap(err, `Failed to write to "scripts/build_rootfs.sh"`)
	}

	_, err = lxd.RunCommand("sh", "build.sh")
	if err != nil {
		return errors.Wrap(err, `Failed to run "build.sh"`)
	}

	return nil
}

func (s *openwrt) getLatestServiceRelease(baseURL, release string) (string, error) {
	resp, err := http.Get(baseURL)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to GET %q", baseURL)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to ready body")
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
		return "", errors.Wrapf(err, "Failed to GET %q", baseURL)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read body")
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
