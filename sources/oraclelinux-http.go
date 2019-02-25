package sources

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"gopkg.in/antchfx/htmlquery.v1"

	"github.com/lxc/distrobuilder/shared"
)

// OracleLinuxHTTP represents the Oracle Linux downloader.
type OracleLinuxHTTP struct {
	majorVersion string
}

// NewOracleLinuxHTTP creates a new OracleLinuxHTTP instance.
func NewOracleLinuxHTTP() *OracleLinuxHTTP {
	return &OracleLinuxHTTP{}
}

// Run downloads Oracle Linux.
func (s *OracleLinuxHTTP) Run(definition shared.Definition, rootfsDir string) error {
	s.majorVersion = definition.Image.Release
	fname := "x86_64-boot.iso"
	baseURL := fmt.Sprintf("%s/OL%s", definition.Source.URL, definition.Image.Release)

	latestUpdate, err := s.getLatestUpdate(baseURL)
	if err != nil {
		return err
	}

	err = shared.DownloadHash(fmt.Sprintf("%s/u%s/x86_64/%s", baseURL, latestUpdate, fname),
		"", nil)
	if err != nil {
		return fmt.Errorf("Error downloading Oracle Linux image: %s", err)
	}

	return s.unpackISO(latestUpdate, filepath.Join(os.TempDir(), fname), rootfsDir)
}

func (s *OracleLinuxHTTP) unpackISO(latestUpdate, filePath, rootfsDir string) error {
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
	defer syscall.Unmount(isoDir, 0)

	var rootfsImage string
	squashfsImage := filepath.Join(isoDir, "LiveOS", "squashfs.img")

	// The squashfs.img contains an image containing the rootfs, so first
	// mount squashfs.img
	err = shared.RunCommand("mount", "-o", "ro", squashfsImage, squashfsDir)
	if err != nil {
		return err
	}
	defer syscall.Unmount(squashfsDir, 0)

	rootfsImage = filepath.Join(squashfsDir, "LiveOS", "rootfs.img")

	err = shared.RunCommand("mount", "-o", "ro", rootfsImage, roRootDir)
	if err != nil {
		return err
	}
	defer syscall.Unmount(roRootDir, 0)

	// Remove rootfsDir otherwise rsync will copy the content into the directory
	// itself
	err = os.RemoveAll(rootfsDir)
	if err != nil {
		return err
	}

	// Since roRootDir is read-only, we need to copy it to a temporary rootfs
	// directory in order to create the minimal rootfs.
	err = shared.RunCommand("rsync", "-qa", roRootDir+"/", tempRootDir)
	if err != nil {
		return err
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(tempRootDir, shared.DefinitionEnv{})
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %s", err)
	}

	err = shared.RunScript(fmt.Sprintf(`
#!/bin/sh
set -eux

version="%s"
update="%s"

# Create required files
touch /etc/mtab /etc/fstab

# Fetch and install rpm and yum from the Oracle repo
_rpm=$(curl -s https://yum.oracle.com/repo/OracleLinux/OL${version}/${update}/base/x86_64/index.html | grep -Eo '>rpm-[[:digit:]][^ ]+\.rpm<' | tail -1 | tr -d '<>')
_yum=$(curl -s https://yum.oracle.com/repo/OracleLinux/OL${version}/${update}/base/x86_64/index.html | grep -Eo '>yum-[[:digit:]][^ ]+\.rpm<' | tail -1 | tr -d '<>')

wget https://yum.oracle.com/repo/OracleLinux/OL${version}/${update}/base/x86_64/getPackage/${_rpm}
wget https://yum.oracle.com/repo/OracleLinux/OL${version}/${update}/base/x86_64/getPackage/${_yum}

# There's no OL7 key!
wget https://oss.oracle.com/ol6/RPM-GPG-KEY-oracle

rpm -ivh --nodeps "${_rpm}" "${_yum}"
rpm --import RPM-GPG-KEY-oracle

# Add repo
mkdir -p /etc/yum.repos.d
cat <<- EOF > /etc/yum.repos.d/base.repo
[base]
name=Oracle Linux
baseurl=https://yum.oracle.com/repo/OracleLinux/OL${version}/${update}/base/x86_64
enabled=1
gpgcheck=1
gpgkey=file:///RPM-GPG-KEY-oracle
EOF

mkdir /rootfs
yum --installroot=/rootfs -y --releasever=${version} install basesystem oraclelinux-release yum
rm -rf /rootfs/var/cache/yum

cp RPM-GPG-KEY-oracle /rootfs

mkdir -p /rootfs/etc/yum.repos.d
cat <<- EOF > /rootfs/etc/yum.repos.d/base.repo
[base]
name=Oracle Linux
baseurl=https://yum.oracle.com/repo/OracleLinux/OL${version}/${update}/base/x86_64
enabled=1
gpgcheck=1
gpgkey=file:///RPM-GPG-KEY-oracle
EOF
`, s.majorVersion, latestUpdate))
	if err != nil {
		exitChroot()
		return err
	}

	exitChroot()

	return shared.RunCommand("rsync", "-qa", tempRootDir+"/rootfs/", rootfsDir)
}

func (s *OracleLinuxHTTP) getLatestUpdate(URL string) (string, error) {
	re := regexp.MustCompile(`^u\d+/$`)

	doc, err := htmlquery.LoadURL(URL)
	if err != nil {
		return "", err
	}

	var latestUpdate string

	for _, a := range htmlquery.Find(doc, "//a/@href") {
		if re.MatchString(a.FirstChild.Data) {
			latestUpdate = a.FirstChild.Data
		}
	}

	if latestUpdate == "" {
		return "", fmt.Errorf("No update found")
	}

	latestUpdate = strings.TrimPrefix(latestUpdate, "u")
	return strings.TrimSuffix(latestUpdate, "/"), nil
}
