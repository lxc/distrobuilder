package sources

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"golang.org/x/sys/unix"
	"gopkg.in/antchfx/htmlquery.v1"

	"github.com/lxc/distrobuilder/shared"
)

type oraclelinux struct {
	commonRHEL

	majorVersion string
	architecture string
}

// Run downloads Oracle Linux.
func (s *oraclelinux) Run() error {
	s.majorVersion = s.definition.Image.Release
	s.architecture = s.definition.Image.ArchitectureMapped

	baseURL := fmt.Sprintf("%s/OL%s", s.definition.Source.URL, s.definition.Image.Release)

	updates, err := s.getUpdates(baseURL)
	if err != nil {
		return fmt.Errorf("Failed to get updates: %w", err)
	}

	var latestUpdate string
	var fname string

	// Only consider updates providing a boot image since we're not interested in the
	// DVD ISO.
	for i := len(updates) - 1; i > 0; i-- {
		URL := fmt.Sprintf("%s/%s/%s", baseURL, updates[i], s.architecture)

		fname, err = s.getISO(URL, s.architecture)
		if err != nil {
			continue
		}

		fullURL := fmt.Sprintf("%s/%s", URL, fname)

		var (
			resp *http.Response
			err  error
		)

		err = shared.Retry(func() error {
			resp, err = http.Head(fullURL)
			if err != nil {
				return errors.New("")
			}

			return nil
		}, 3)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			latestUpdate = updates[i]
			break
		}
	}

	source := fmt.Sprintf("%s/%s/%s/%s", baseURL, latestUpdate, s.architecture, fname)

	fpath, err := shared.DownloadHash(s.definition.Image, source, "", nil)
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", source, err)
	}

	s.logger.Infow("Unpacking ISO", "file", filepath.Join(fpath, fname))

	err = s.unpackISO(latestUpdate[1:], filepath.Join(fpath, fname), s.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed to unpack ISO: %w", err)
	}

	return nil
}

func (s *oraclelinux) unpackISO(latestUpdate, filePath, rootfsDir string) error {
	isoDir := filepath.Join(s.cacheDir, "iso")
	squashfsDir := filepath.Join(s.cacheDir, "squashfs")
	roRootDir := filepath.Join(s.cacheDir, "rootfs.ro")
	tempRootDir := filepath.Join(s.cacheDir, "rootfs")

	for _, dir := range []string{isoDir, squashfsDir, roRootDir} {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create %q: %w", dir, err)
		}
	}

	// this is easier than doing the whole loop thing ourselves
	err := shared.RunCommand("mount", "-o", "ro", filePath, isoDir)
	if err != nil {
		return fmt.Errorf("Failed to mount %q: %w", filePath, err)
	}
	defer unix.Unmount(isoDir, 0)

	var rootfsImage string
	squashfsImage := filepath.Join(isoDir, "LiveOS", "squashfs.img")
	if lxd.PathExists(squashfsImage) {

		// The squashfs.img contains an image containing the rootfs, so first
		// mount squashfs.img
		err = shared.RunCommand("mount", "-o", "ro", squashfsImage, squashfsDir)
		if err != nil {
			return fmt.Errorf("Failed to mount %q: %w", squashfsImage, err)
		}
		defer unix.Unmount(squashfsDir, 0)

		rootfsImage = filepath.Join(squashfsDir, "LiveOS", "rootfs.img")
	} else {
		rootfsImage = filepath.Join(isoDir, "images", "install.img")
	}

	// Remove rootfsDir otherwise rsync will copy the content into the directory
	// itself
	err = os.RemoveAll(rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed to remove %q: %w", rootfsDir, err)
	}

	s.logger.Infow("Unpacking root image", "file", rootfsImage)

	err = s.unpackRootfsImage(rootfsImage, tempRootDir)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", rootfsImage, err)
	}

	// Determine rpm and yum packages
	var baseURL string

	if s.majorVersion == "7" {
		baseURL = fmt.Sprintf("https://yum.oracle.com/repo/OracleLinux/OL%s/%s/base/%s", s.majorVersion, latestUpdate, s.architecture)
	} else {
		baseURL = fmt.Sprintf("https://yum.oracle.com/repo/OracleLinux/OL%s/%s/baseos/base/%s", s.majorVersion, latestUpdate, s.architecture)
	}

	doc, err := htmlquery.LoadURL(fmt.Sprintf("%s/index.html", baseURL))
	if err != nil {
		return fmt.Errorf("Failed to load URL %q: %w", fmt.Sprintf("%s/index.html", baseURL), err)
	}

	regexRpm := regexp.MustCompile(`^getPackage/rpm-\d+.+\.rpm$`)
	regexYum := regexp.MustCompile(`^getPackage/yum-\d+.+\.rpm$`)

	var yumPkgs []string
	var rpmPkgs []string

	for _, a := range htmlquery.Find(doc, `//a/@href`) {
		if regexRpm.MatchString(a.FirstChild.Data) {
			rpmPkgs = append(rpmPkgs, a.FirstChild.Data)
			continue
		}

		if regexYum.MatchString(a.FirstChild.Data) {
			yumPkgs = append(yumPkgs, a.FirstChild.Data)
			continue
		}
	}

	sort.Strings(yumPkgs)
	sort.Strings(rpmPkgs)

	if len(rpmPkgs) > 0 && len(yumPkgs) > 0 {
		yumPkg := yumPkgs[len(yumPkgs)-1]
		rpmPkg := rpmPkgs[len(rpmPkgs)-1]

		array := [][]string{
			{filepath.Join(tempRootDir, filepath.Base(rpmPkg)), fmt.Sprintf("%s/%s", baseURL, rpmPkg)},
			{filepath.Join(tempRootDir, filepath.Base(yumPkg)), fmt.Sprintf("%s/%s", baseURL, yumPkg)},
			{filepath.Join(tempRootDir, "RPM-GPG-KEY-oracle"), "https://oss.oracle.com/ol6/RPM-GPG-KEY-oracle"},
		}

		for _, elem := range array {
			f, err := os.Create(elem[0])
			if err != nil {
				return fmt.Errorf("Failed to create file %q: %w", elem[0], err)
			}
			defer f.Close()

			_, err = lxd.DownloadFileHash(http.DefaultClient, "", nil, nil, elem[0], elem[1], "", nil, f)
			if err != nil {
				return fmt.Errorf("Failed to download %q: %w", elem[1], err)
			}
			f.Close()
		}
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(tempRootDir, shared.DefinitionEnv{}, nil)
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %w", err)
	}

	if !lxd.PathExists("/bin") && lxd.PathExists("/usr/bin") {
		err = os.Symlink("/usr/bin", "/bin")
		if err != nil {
			return fmt.Errorf("Failed to create /bin symlink: %w", err)
		}
	}

	err = shared.RunScript(fmt.Sprintf(`#!/bin/sh
set -eux

version="%s"
latest_update="%s"
arch="%s"

# Create required files
touch /etc/mtab /etc/fstab

mkdir -p /etc/yum.repos.d /rootfs

if [ "${version}" = "7" ]; then
	baseurl=https://yum.oracle.com/repo/OracleLinux/OL${version}/${latest_update}/base/${arch}/
	baseurl_latest=https://yum.oracle.com/repo/OracleLinux/OL${version}/latest/${arch}/
else
	baseurl=https://yum.oracle.com/repo/OracleLinux/OL${version}/${latest_update}/baseos/base/${arch}/
	baseurl_latest=https://yum.oracle.com/repo/OracleLinux/OL${version}/baseos/latest/${arch}/
fi

if which dnf; then
	alias yum=dnf
	gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-oracle
else
	gpgkey=file:///RPM-GPG-KEY-oracle

	# Fetch and install rpm and yum from the Oracle repo
	_rpm=$(curl -s ${baseurl}/index.html | grep -Eo '>rpm-[[:digit:]][^ ]+\.rpm<' | tail -1 | sed 's|[<>]||g')
	_yum=$(curl -s ${baseurl}/index.html | grep -Eo '>yum-[[:digit:]][^ ]+\.rpm<' | tail -1 | sed 's|[<>]||g')

	rpm -ivh --nodeps "${_rpm}" "${_yum}"
	rpm --import RPM-GPG-KEY-oracle
fi

# Add repo
cat <<- EOF > /etc/yum.repos.d/base.repo
[base]
name=Oracle Linux
baseurl=${baseurl}
enabled=1
gpgcheck=1
gpgkey=${gpgkey}
EOF

cat <<- EOF > /etc/yum.repos.d/base_latest.repo
[latest]
name=Oracle Linux
baseurl=${baseurl_latest}
enabled=1
gpgcheck=1
gpgkey=${gpgkey}
EOF

rm -rf /var/rootfs/*

yum install --disablerepo=latest --releasever=${version} --installroot=/rootfs -y basesystem oraclelinux-release

if [ "${version}" = "7" ] && [ "${arch}" = "aarch64" ]; then
	yum install --releasever=${version} --installroot=/rootfs -y libcom_err
fi

yum install --disablerepo=latest --releasever=${version} --installroot=/rootfs -y yum

rm -rf /rootfs/var/cache/yum

mkdir -p /rootfs/etc/yum.repos.d
cp /etc/yum.repos.d/base.repo /rootfs/etc/yum.repos.d/

if [ -f RPM-GPG-KEY-oracle ] && ! [ -f /rootfs/etc/pki/rpm-gpg/RPM-GPG-KEY-oracle ]; then
	mkdir -p /rootfs/etc/pki/rpm-gpg/
	cp RPM-GPG-KEY-oracle /rootfs/etc/pki/rpm-gpg/
fi

cat <<- EOF > /rootfs/etc/yum.repos.d/base.repo
[base]
name=Oracle Linux
baseurl=${baseurl}
enabled=1
gpgcheck=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-oracle
EOF

`, s.majorVersion, latestUpdate, s.architecture))
	if err != nil {
		exitChroot()
		return fmt.Errorf("Failed to run script: %w", err)
	}

	exitChroot()

	err = shared.RsyncLocal(tempRootDir+"/rootfs/", rootfsDir)
	if err != nil {
		return fmt.Errorf(`Failed to run "rsync": %w`, err)
	}

	return nil
}

func (s *oraclelinux) getISO(URL string, architecture string) (string, error) {
	var re *regexp.Regexp

	if architecture == "x86_64" {
		re = regexp.MustCompile(fmt.Sprintf("%s-boot(-\\d{8})?.iso", architecture))
	} else if architecture == "aarch64" {
		re = regexp.MustCompile(fmt.Sprintf("%s-boot-uek(-\\d{8})?.iso", architecture))
	} else {
		return "", fmt.Errorf("Unsupported architecture %q", architecture)
	}

	doc, err := htmlquery.LoadURL(URL)
	if err != nil {
		return "", fmt.Errorf("Failed to load URL %q: %w", URL, err)
	}

	var isos []string

	for _, a := range htmlquery.Find(doc, "//a/@href") {
		if re.MatchString(a.FirstChild.Data) {
			isos = append(isos, a.FirstChild.Data)
		}
	}

	if len(isos) == 0 {
		return "", errors.New("No isos found")
	}

	return isos[len(isos)-1], nil
}

func (s *oraclelinux) getUpdates(URL string) ([]string, error) {
	re := regexp.MustCompile(`^[uU]\d+/$`)

	doc, err := htmlquery.LoadURL(URL)
	if err != nil {
		return nil, fmt.Errorf("Failed to load URL %q: %w", URL, err)
	}

	var updates []string

	for _, a := range htmlquery.Find(doc, "//a/@href") {
		if re.MatchString(a.FirstChild.Data) {
			updates = append(updates, strings.TrimSuffix(a.FirstChild.Data, "/"))
		}
	}

	if len(updates) == 0 {
		return nil, errors.New("No updates found")
	}

	return updates, nil
}
