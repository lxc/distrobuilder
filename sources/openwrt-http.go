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

// OpenWrtHTTP represents the ALT Linux downloader.
type OpenWrtHTTP struct{}

// NewOpenWrtHTTP creates a new OpenWrtHTTP instance.
func NewOpenWrtHTTP() *OpenWrtHTTP {
	return &OpenWrtHTTP{}
}

// Run downloads the tarball and unpacks it.
func (s *OpenWrtHTTP) Run(definition shared.Definition, rootfsDir string) error {
	var baseURL string

	release := definition.Image.Release
	releaseInFilename := strings.ToLower(release) + "-"
	architecturePath := strings.Replace(definition.Image.ArchitectureMapped, "_", "/", 1)

	// Figure out the correct release
	if release == "snapshot" {
		// Build a daily snapshot.
		baseURL = fmt.Sprintf("%s/snapshots/targets/%s/",
			definition.Source.URL, architecturePath)
		releaseInFilename = ""
	} else {
		baseURL = fmt.Sprintf("%s/releases", definition.Source.URL)

		matched, err := regexp.MatchString(`^\d+\.\d+$`, release)
		if err != nil {
			return err
		}

		if matched {
			// A release of the form '18.06' has been provided. We need to find
			// out the latest service release of the form '18.06.0'.
			release = s.getLatestServiceRelease(baseURL, release)
			releaseInFilename = strings.ToLower(release) + "-"
		}

		baseURL = fmt.Sprintf("%s/%s/targets/%s/", baseURL, release, architecturePath)
	}

	fname := fmt.Sprintf("openwrt-%s%s-generic-rootfs.tar.gz", releaseInFilename,
		strings.Replace(definition.Image.ArchitectureMapped, "_", "-", 1))

	url, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	checksumFile := ""
	if !definition.Source.SkipVerification {
		if len(definition.Source.Keys) != 0 {
			checksumFile = baseURL + "sha256sums"
			fpath, err := shared.DownloadHash(definition.Image, checksumFile+".asc", "", nil)
			if err != nil {
				return err
			}

			_, err = shared.DownloadHash(definition.Image, checksumFile, "", nil)
			if err != nil {
				return err
			}

			valid, err := shared.VerifyFile(
				filepath.Join(fpath, "sha256sums"),
				filepath.Join(fpath, "sha256sums.asc"),
				definition.Source.Keys,
				definition.Source.Keyserver)
			if err != nil {
				return err
			}
			if !valid {
				return fmt.Errorf("Failed to validate archive")
			}
		} else {
			// Force gpg checks when using http
			if url.Scheme != "https" {
				return errors.New("GPG keys are required if downloading from HTTP")
			}
		}
	}

	fpath, err := shared.DownloadHash(definition.Image, baseURL+fname, checksumFile, sha256.New())
	if err != nil {
		return err
	}

	sdk := s.getSDK(baseURL, release)
	if sdk == "" {
		return fmt.Errorf("Failed to find SDK")
	}

	_, err = shared.DownloadHash(definition.Image, baseURL+sdk, checksumFile, sha256.New())
	if err != nil {
		return err
	}

	_, err = shared.DownloadHash(definition.Image, "https://github.com/mikma/lxd-openwrt/archive/master.tar.gz", "", sha256.New())
	if err != nil {
		return err
	}

	tempScriptsDir := filepath.Join(os.TempDir(), "distrobuilder", "fixes", "lxd-openwrt-master")
	tempSDKDir := filepath.Join(tempScriptsDir, "build_dir")

	os.MkdirAll(tempSDKDir, 0755)
	os.MkdirAll(tempScriptsDir, 0755)
	defer os.RemoveAll(filepath.Join(os.TempDir(), "distrobuilder"))

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), rootfsDir, false, false, nil)
	if err != nil {
		return err
	}

	err = lxd.Unpack(filepath.Join(fpath, "master.tar.gz"), filepath.Join(os.TempDir(), "distrobuilder", "fixes"), false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to unpack scripts: %v", err)
	}

	err = lxd.Unpack(filepath.Join(fpath, sdk), tempSDKDir, false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to unpack SDK: %v", err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(currentDir)

	// Set environment used in the lxd-openwrt scripts
	os.Setenv("OPENWRT_ROOTFS", filepath.Join(fpath, fname))

	// Always use an absolute path
	if strings.HasPrefix(rootfsDir, "/") {
		os.Setenv("OPENWRT_ROOTFS_DIR", rootfsDir)
	} else {
		os.Setenv("OPENWRT_ROOTFS_DIR", filepath.Join(currentDir, rootfsDir))
	}

	os.Setenv("OPENWRT_SDK", fmt.Sprintf("build_dir/%s", strings.TrimSuffix(sdk, ".tar.xz")))
	os.Setenv("OPENWRT_ARCH", definition.Image.Architecture)
	os.Setenv("OPENWRT_VERSION", release)

	err = os.Chdir(tempScriptsDir)
	if err != nil {
		return err
	}

	f, err := os.Open("build.sh")
	if err != nil {
		return err
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
		return err
	}

	f, err = os.Open("scripts/build_rootfs.sh")
	if err != nil {
		return err
	}

	newContent.Reset()

	scanner = bufio.NewScanner(f)

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "dir=") {
			newContent.WriteString("dir=/tmp/distrobuilder\n")
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

	err = ioutil.WriteFile("scripts/build_build.sh", []byte(newContent.String()), 0755)
	if err != nil {
		return err
	}

	_, err = lxd.RunCommand("sh", "build.sh")
	if err != nil {
		return err
	}

	return nil
}

func (s *OpenWrtHTTP) getLatestServiceRelease(baseURL, release string) string {
	resp, err := http.Get(baseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}

	regex := regexp.MustCompile(fmt.Sprintf(">(%s\\.\\d+)<", release))
	releases := regex.FindAllStringSubmatch(string(body), -1)

	if len(releases) > 0 {
		return releases[len(releases)-1][1]
	}

	return ""
}

func (s *OpenWrtHTTP) getSDK(baseURL, release string) string {
	resp, err := http.Get(baseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}

	if release == "snapshot" {
		release = ""
	} else {
		release = fmt.Sprintf("-%s", release)
	}

	regex := regexp.MustCompile(fmt.Sprintf(">(openwrt-sdk%s-.*\\.tar\\.xz)<", release))
	releases := regex.FindAllStringSubmatch(string(body), -1)

	if len(releases) > 0 {
		return releases[len(releases)-1][1]
	}

	return ""
}
