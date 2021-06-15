package sources

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"gopkg.in/antchfx/htmlquery.v1"

	"github.com/lxc/distrobuilder/shared"
)

type oraclelinux struct {
	common

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
		return err
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

		resp, err := http.Head(fullURL)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			latestUpdate = updates[i]
			break
		}
	}

	fpath, err := shared.DownloadHash(s.definition.Image, fmt.Sprintf("%s/%s/%s/%s", baseURL, latestUpdate, s.architecture, fname),
		"", nil)
	if err != nil {
		return errors.Wrap(err, "Error downloading Oracle Linux image")
	}

	return s.unpackISO(latestUpdate[1:], filepath.Join(fpath, fname), s.rootfsDir)
}

func (s *oraclelinux) unpackISO(latestUpdate, filePath, rootfsDir string) error {
	isoDir := filepath.Join(os.TempDir(), "distrobuilder", "iso")
	squashfsDir := filepath.Join(os.TempDir(), "distrobuilder", "squashfs")
	roRootDir := filepath.Join(os.TempDir(), "distrobuilder", "rootfs.ro")
	tempRootDir := filepath.Join(os.TempDir(), "distrobuilder", "rootfs")

	os.MkdirAll(isoDir, 0755)
	os.MkdirAll(squashfsDir, 0755)
	os.MkdirAll(roRootDir, 0755)
	defer os.RemoveAll(filepath.Join(os.TempDir(), "distrobuilder"))

	// this is easier than doing the whole loop thing ourselves
	err := shared.RunCommand("mount", "-o", "ro", filePath, isoDir)
	if err != nil {
		return err
	}
	defer unix.Unmount(isoDir, 0)

	var rootfsImage string
	squashfsImage := filepath.Join(isoDir, "LiveOS", "squashfs.img")
	if lxd.PathExists(squashfsImage) {

		// The squashfs.img contains an image containing the rootfs, so first
		// mount squashfs.img
		err = shared.RunCommand("mount", "-o", "ro", squashfsImage, squashfsDir)
		if err != nil {
			return err
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
		return err
	}

	err = s.unpackRootfsImage(rootfsImage, tempRootDir)
	if err != nil {
		return err
	}

	// Determine rpm and yum packages
	baseURL := fmt.Sprintf("https://yum.oracle.com/repo/OracleLinux/OL%s/%s/base/%s", s.majorVersion, latestUpdate, s.architecture)

	doc, err := htmlquery.LoadURL(fmt.Sprintf("%s/index.html", baseURL))
	if err != nil {
		return err
	}

	regexRpm := regexp.MustCompile(`^getPackage/rpm-\d+.+\.rpm$`)
	regexYum := regexp.MustCompile(`^getPackage/yum-\d+.+\.rpm$`)

	var yumPkg string
	var rpmPkg string

	for _, a := range htmlquery.Find(doc, `//a/@href`) {
		if rpmPkg == "" && regexRpm.MatchString(a.FirstChild.Data) {
			rpmPkg = a.FirstChild.Data
			continue
		}

		if yumPkg == "" && regexYum.MatchString(a.FirstChild.Data) {
			yumPkg = a.FirstChild.Data
			continue
		}

		if rpmPkg != "" && yumPkg != "" {
			break
		}
	}

	if rpmPkg != "" && yumPkg != "" {
		rpmFileName := filepath.Join(tempRootDir, filepath.Base(rpmPkg))
		yumFileName := filepath.Join(tempRootDir, filepath.Base(yumPkg))
		gpgFileName := filepath.Join(tempRootDir, "RPM-GPG-KEY-oracle")

		rpmFile, err := os.Create(rpmFileName)
		if err != nil {
			return err
		}
		defer rpmFile.Close()

		yumFile, err := os.Create(yumFileName)
		if err != nil {
			return err
		}
		defer yumFile.Close()

		gpgFile, err := os.Create(gpgFileName)
		if err != nil {
			return err
		}
		defer gpgFile.Close()

		_, err = lxd.DownloadFileHash(http.DefaultClient, "", nil, nil, rpmFileName, fmt.Sprintf("%s/%s", baseURL, rpmPkg), "", nil, rpmFile)
		if err != nil {
			return err
		}
		rpmFile.Close()

		_, err = lxd.DownloadFileHash(http.DefaultClient, "", nil, nil, yumFileName, fmt.Sprintf("%s/%s", baseURL, yumPkg), "", nil, yumFile)
		if err != nil {
			return err
		}
		yumFile.Close()

		_, err = lxd.DownloadFileHash(http.DefaultClient, "", nil, nil, gpgFileName, "https://oss.oracle.com/ol6/RPM-GPG-KEY-oracle", "", nil, gpgFile)
		if err != nil {
			return err
		}
		gpgFile.Close()
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(tempRootDir, shared.DefinitionEnv{}, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to setup chroot")
	}

	if !lxd.PathExists("/bin") && lxd.PathExists("/usr/bin") {
		err = os.Symlink("/usr/bin", "/bin")
		if err != nil {
			return errors.Wrap(err, "Failed to create /bin symlink")
		}
	}

	err = shared.RunScript(fmt.Sprintf(`#!/bin/sh
set -eux

version="%s"
update="%s"
arch="%s"

# Create required files
touch /etc/mtab /etc/fstab

mkdir -p /etc/yum.repos.d /rootfs

if which dnf; then
	alias yum=dnf
	baseurl=http://yum.oracle.com/repo/OracleLinux/OL${version}/baseos/latest/${arch}/
	gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-oracle
else
	baseurl=http://yum.oracle.com/repo/OracleLinux/OL${version}/${update}/base/${arch}
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

rm -rf /var/rootfs/*

yum install --releasever=${version} --installroot=/rootfs -y basesystem oraclelinux-release yum
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
		return err
	}

	exitChroot()

	return shared.RunCommand("rsync", "-qa", tempRootDir+"/rootfs/", rootfsDir)
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
		return "", err
	}

	var isos []string

	for _, a := range htmlquery.Find(doc, "//a/@href") {
		if re.MatchString(a.FirstChild.Data) {
			isos = append(isos, a.FirstChild.Data)
		}
	}

	if len(isos) == 0 {
		return "", fmt.Errorf("No isos found")
	}

	return isos[len(isos)-1], nil
}

func (s *oraclelinux) getUpdates(URL string) ([]string, error) {
	re := regexp.MustCompile(`^[uU]\d+/$`)

	doc, err := htmlquery.LoadURL(URL)
	if err != nil {
		return nil, err
	}

	var updates []string

	for _, a := range htmlquery.Find(doc, "//a/@href") {
		if re.MatchString(a.FirstChild.Data) {
			updates = append(updates, strings.TrimSuffix(a.FirstChild.Data, "/"))
		}
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("No updates found")
	}

	return updates, nil
}

func (s oraclelinux) unpackRootfsImage(imageFile string, target string) error {
	installDir, err := ioutil.TempDir(filepath.Join(os.TempDir(), "distrobuilder"), "temp_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(installDir)

	err = shared.RunCommand("mount", "-o", "ro", imageFile, installDir)
	if err != nil {
		return err
	}
	defer unix.Unmount(installDir, 0)

	rootfsDir := installDir
	rootfsFile := filepath.Join(installDir, "LiveOS", "rootfs.img")

	if lxd.PathExists(rootfsFile) {
		rootfsDir, err = ioutil.TempDir(filepath.Join(os.TempDir(), "distrobuilder"), "temp_")
		if err != nil {
			return err
		}
		defer os.RemoveAll(rootfsDir)

		err = shared.RunCommand("mount", "-o", "ro", rootfsFile, rootfsDir)
		if err != nil {
			return err
		}
		defer unix.Unmount(rootfsFile, 0)
	}

	// Since rootfs is read-only, we need to copy it to a temporary rootfs
	// directory in order to create the minimal rootfs.
	return shared.RunCommand("rsync", "-qa", rootfsDir+"/", target)
}
