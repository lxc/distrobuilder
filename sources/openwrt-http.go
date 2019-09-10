package sources

import (
	"bytes"
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

	diff := `diff --git a/build.sh b/build.sh
index 2347d05..eebb515 100755
--- a/build.sh
+++ b/build.sh
@@ -2,8 +2,8 @@

 set -e

-arch_lxd=x86_64
-ver=18.06.4
+arch_lxd=${OPENWRT_ARCH}
+ver=${OPENWRT_VERSION}
 dist=openwrt
 type=lxd
 super=fakeroot
@@ -13,6 +13,9 @@ packages=iptables-mod-checksum
 # Workaround for Debian/Ubuntu systems which use C.UTF-8 which is nsupported by OpenWrt
 export LC_ALL=C

+readonly rootfs=${OPENWRT_ROOTFS}
+readonly sdk=${OPENWRT_SDK}
+
 usage() {
	 echo "Usage: $0 [-a|--arch x86_64|i686|aarch64] [-v|--version version>] [-p|--packages <packages>] [-f|--files] [-t|--type lxd|lain] [-s|--super fakeroot|sudo] [--help]"
	 exit 1
@@ -289,8 +292,6 @@ EOF
 #     template: hostname.tpl
 }

-download_rootfs
-download_sdk
 if need_procd; then
	 download_procd
	 build_procd
diff --git a/scripts/build_rootfs.sh b/scripts/build_rootfs.sh
index b7ee533..e89379f 100755
--- a/scripts/build_rootfs.sh
+++ b/scripts/build_rootfs.sh
@@ -52,9 +52,9 @@ fi

 src_tar=$1
 base=` + "`basename $src_tar`" + `
-dir=/tmp/build.$$
+dir=/tmp/distrobuilder
 files_dir=files/
-instroot=$dir/rootfs
+instroot=${OPENWRT_ROOTFS_DIR}
 cache=dl/packages/$arch/$subarch

 test -e $cache || mkdir -p $cache
@@ -158,7 +158,6 @@ create_manifest() {
	 $OPKG list-installed > $instroot/etc/openwrt_manifest
 }

-unpack
 disable_root
 if test -n "$metadata"; then
	 add_file $metadata $metadata_dir $dir
@@ -175,5 +174,3 @@ if test -n "$files"; then
	 add_files $files $instroot
 fi
 create_manifest
-pack
-#pack_squashfs
`

	err = os.Chdir(tempScriptsDir)
	if err != nil {
		return err
	}

	err = lxd.RunCommandWithFds(bytes.NewBufferString(diff), os.Stdout, "patch", "-p1")
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
