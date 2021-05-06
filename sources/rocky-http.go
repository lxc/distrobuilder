package sources

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/shared"
)

// RockyLinuxHTTP represents the Rocky HTTP downloader.
type RockyLinuxHTTP struct {
	fname        string
	majorVersion string
}

// NewRockyLinuxHTTP creates a new RockyOSHTTP instance.
func NewRockyLinuxHTTP() *RockyLinuxHTTP {
	return &RockyLinuxHTTP{}
}

// Run downloads the tarball and unpacks it.
func (s *RockyLinuxHTTP) Run(definition shared.Definition, rootfsDir string) error {

	s.majorVersion = strings.Split(definition.Image.Release, ".")[0]

	baseURL := fmt.Sprintf("%s/%s/isos/%s/", definition.Source.URL,
		strings.ToLower(definition.Image.Release),
		definition.Image.ArchitectureMapped)
	s.fname = s.getRelease(definition.Source.URL, definition.Image.Release,
		definition.Source.Variant, definition.Image.ArchitectureMapped)
	if s.fname == "" {
		return fmt.Errorf("Couldn't get name of iso")
	}

	fpath := shared.GetTargetDir(definition.Image)

	url, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	checksumFile := ""
	if !definition.Source.SkipVerification {
		// Force gpg checks when using http
		if url.Scheme != "https" {
			if len(definition.Source.Keys) == 0 {
				return errors.New("GPG keys are required if downloading from HTTP")
			}

			checksumFile = "CHECKSUM"

			_, err := shared.DownloadHash(definition.Image, baseURL+checksumFile, "", nil)
			if err != nil {
				return err
			}
		}
	}

	_, err = shared.DownloadHash(definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return errors.Wrap(err, "Error downloading RockyOS image")
	}

	return s.unpackISO(filepath.Join(fpath, s.fname), rootfsDir)
}

func (s *RockyLinuxHTTP) unpackISO(filePath, rootfsDir string) error {
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

	rootfsImage := filepath.Join(isoDir, "images", "install.img")

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

	gpgKeysPath := ""

	packagesDir := filepath.Join(isoDir, "Packages")
	repodataDir := filepath.Join(isoDir, "repodata")

	if !lxd.PathExists(packagesDir) {
		packagesDir = filepath.Join(isoDir, "BaseOS", "Packages")
	}
	if !lxd.PathExists(repodataDir) {
		repodataDir = filepath.Join(isoDir, "BaseOS", "repodata")
	}

	if lxd.PathExists(packagesDir) && lxd.PathExists(repodataDir) {
		// Create cdrom repo for yum
		err = os.MkdirAll(filepath.Join(tempRootDir, "mnt", "cdrom"), 0755)
		if err != nil {
			return err
		}

		// Copy repo relevant files to the cdrom
		err = shared.RunCommand("rsync", "-qa",
			packagesDir,
			repodataDir,
			filepath.Join(tempRootDir, "mnt", "cdrom"))
		if err != nil {
			return err
		}

		// Find all relevant GPG keys
		gpgKeys, err := filepath.Glob(filepath.Join(isoDir, "RPM-GPG-KEY-*"))
		if err != nil {
			return err
		}

		// Copy the keys to the cdrom
		for _, key := range gpgKeys {
			fmt.Printf("key=%v\n", key)
			if len(gpgKeysPath) > 0 {
				gpgKeysPath += " "
			}
			gpgKeysPath += fmt.Sprintf("file:///mnt/cdrom/%s", filepath.Base(key))

			err = shared.RunCommand("rsync", "-qa", key,
				filepath.Join(tempRootDir, "mnt", "cdrom"))
			if err != nil {
				return err
			}
		}
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(tempRootDir, shared.DefinitionEnv{}, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to setup chroot")
	}

	err = shared.RunScript(fmt.Sprintf(`#!/bin/sh
set -eux
GPG_KEYS="%s"
RELEASE="%s"
# Create required files
touch /etc/mtab /etc/fstab
yum_args=""
mkdir -p /etc/yum.repos.d
if [ -d /mnt/cdrom ]; then
	# Install initial package set
	cd /mnt/cdrom/Packages
	rpm -ivh --nodeps $(ls yum-*.rpm | head -n1)
	# Add cdrom repo
	cat <<- EOF > /etc/yum.repos.d/cdrom.repo
[cdrom]
name=Install CD-ROM
baseurl=file:///mnt/cdrom
enabled=0
EOF
	if [ -n "${GPG_KEYS}" ]; then
		echo gpgcheck=1 >> /etc/yum.repos.d/cdrom.repo
		echo gpgkey=${GPG_KEYS} >> /etc/yum.repos.d/cdrom.repo
	else
		echo gpgcheck=0 >> /etc/yum.repos.d/cdrom.repo
	fi
	yum_args="--disablerepo=* --enablerepo=cdrom"
	yum ${yum_args} -y --releasever="${RELEASE}" reinstall yum
else
	if ! [ -f /etc/pki/rpm-gpg/RPM-GPG-KEY-rockyofficial ]; then
		mkdir -p /etc/pki/rpm-gpg
		cat <<- "EOF" > /etc/pki/rpm-gpg/RPM-GPG-KEY-rockyofficial
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v2.0.22 (GNU/Linux)
mQINBGAofzYBEAC6yS1azw6f3wmaVd//3aSy6O2c9+jeetulRQvg2LvhRRS1eNqp
/x9tbBhfohu/tlDkGpYHV7diePgMml9SZDy1sKlI3tDhx6GZ3xwF0fd1vWBZpmNk
D9gRkUmYBeLotmcXQZ8ZpWLicosFtDpJEYpLUhuIgTKwt4gxJrHvkWsGQiBkJxKD
u3/RlL4IYA3Ot9iuCBflc91EyAw1Yj0gKcDzbOqjvlGtS3ASXgxPqSfU0uLC9USF
uKDnP2tcnlKKGfj0u6VkqISliSuRAzjlKho9Meond+mMIFOTT6qp4xyu+9Dj3IjZ
IC6rBXRU3xi8z0qYptoFZ6hx70NV5u+0XUzDMXdjQ5S859RYJKijiwmfMC7gZQAf
OkdOcicNzen/TwD/slhiCDssHBNEe86Wwu5kmDoCri7GJlYOlWU42Xi0o1JkVltN
D8ZId+EBDIms7ugSwGOVSxyZs43q2IAfFYCRtyKHFlgHBRe9/KTWPUrnsfKxGJgC
Do3Yb63/IYTvfTJptVfhQtL1AhEAeF1I+buVoJRmBEyYKD9BdU4xQN39VrZKziO3
hDIGng/eK6PaPhUdq6XqvmnsZ2h+KVbyoj4cTo2gKCB2XA7O2HLQsuGduHzYKNjf
QR9j0djjwTrsvGvzfEzchP19723vYf7GdcLvqtPqzpxSX2FNARpCGXBw9wARAQAB
tDNSZWxlYXNlIEVuZ2luZWVyaW5nIDxpbmZyYXN0cnVjdHVyZUByb2NreWxpbnV4
Lm9yZz6JAk4EEwEIADgWIQRwUcRwqSn0VM6+N7cVr12sbXRaYAUCYCh/NgIbDwUL
CQgHAgYVCgkICwIEFgIDAQIeAQIXgAAKCRAVr12sbXRaYLFmEACSMvoO1FDdyAbu
1m6xEzDhs7FgnZeQNzLZECv2j+ggFSJXezlNVOZ5I1I8umBan2ywfKQD8M+IjmrW
k9/7h9i54t8RS/RN7KNo7ECGnKXqXDPzBBTs1Gwo1WzltAoaDKUfXqQ4oJ4aCP/q
/XPVWEzgpJO1XEezvCq8VXisutyDiXEjjMIeBczxb1hbamQX+jLTIQ1MDJ4Zo1YP
zlUqrHW434XC2b1/WbSaylq8Wk9cksca5J+g3FqTlgiWozyy0uxygIRjb6iTzKXk
V7SYxeXp3hNTuoUgiFkjh5/0yKWCwx7aQqlHar9GjpxmBDAO0kzOlgtTw//EqTwR
KnYZLig9FW0PhwvZJUigr0cvs/XXTTb77z/i/dfHkrjVTTYenNyXogPtTtSyxqca
61fbPf0B/S3N43PW8URXBRS0sykpX4SxKu+PwKCqf+OJ7hMEVAapqzTt1q9T7zyB
QwvCVx8s7WWvXbs2d6ZUrArklgjHoHQcdxJKdhuRmD34AuXWCLW+gH8rJWZpuNl3
+WsPZX4PvjKDgMw6YMcV7zhWX6c0SevKtzt7WP3XoKDuPhK1PMGJQqQ7spegGB+5
DZvsJS48Ip0S45Qfmj82ibXaCBJHTNZE8Zs+rdTjQ9DS5qvzRA1sRA1dBb/7OLYE
JmeWf4VZyebm+gc50szsg6Ut2yT8hw==
=AiP8
-----END PGP PUBLIC KEY BLOCK-----
EOF
	fi
	cat <<- "EOF" > /etc/yum.repos.d/Rocky-BaseOS.repo
[BaseOS]
name=Rocky-$releasever - Base
mirrorlist=http://mirrors.rockylinux.org/mirrorlist?arch=$basearch&repo=BaseOS-8
gpgcheck=1
enabled=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-rockyofficial
EOF
	# Use dnf in the boot iso since yum isn't available
	alias yum=dnf
fi
pkgs="basesystem Rocky-release yum"
# Create a minimal rootfs
mkdir /rootfs
yum ${yum_args} --installroot=/rootfs -y --releasever="${RELEASE}" --skip-broken install ${pkgs}
rm -rf /rootfs/var/cache/yum
`, gpgKeysPath, s.majorVersion))
	if err != nil {
		exitChroot()
		return err
	}

	exitChroot()

	return shared.RunCommand("rsync", "-qa", tempRootDir+"/rootfs/", rootfsDir)
}

func (s *RockyLinuxHTTP) getRelease(URL, release, variant, arch string) string {
	resp, err := http.Get(URL + path.Join("/", strings.ToLower(release), "isos", arch))
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

	re := s.getRegexes(arch, variant, release)

	for _, r := range re {
		matches := r.FindAllString(string(body), -1)
		if len(matches) > 0 {
			return matches[len(matches)-1]
		}
	}

	return ""
}

func (s *RockyLinuxHTTP) getRegexes(arch string, variant string, release string) []*regexp.Regexp {
	releaseFields := strings.Split(release, ".")

	var re []string
	switch len(releaseFields) {
	case 1:
		re = append(re, fmt.Sprintf("Rocky-%s(.\\d+)*-%s-(?i:%s).iso",
			releaseFields[0], arch, variant))
	case 2:
		re = append(re, fmt.Sprintf("Rocky-%s.%s-%s-(?i:%s).iso",
			releaseFields[0], releaseFields[1], arch, variant))
	}

	regexes := make([]*regexp.Regexp, len(re))

	for i, r := range re {
		regexes[i] = regexp.MustCompile(r)
	}

	return regexes
}

func (s *RockyLinuxHTTP) unpackRootfsImage(imageFile string, target string) error {
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
