package sources

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/shared"
)

// CentOSHTTP represents the CentOS HTTP downloader.
type CentOSHTTP struct {
	fname        string
	majorVersion string
}

// NewCentOSHTTP creates a new CentOSHTTP instance.
func NewCentOSHTTP() *CentOSHTTP {
	return &CentOSHTTP{}
}

// Run downloads the tarball and unpacks it.
func (s *CentOSHTTP) Run(definition shared.Definition, rootfsDir string) error {
	if strings.HasSuffix(definition.Image.Release, "-Stream") {
		s.majorVersion = strings.ToLower(definition.Image.Release)
	} else {
		s.majorVersion = strings.Split(definition.Image.Release, ".")[0]
	}

	baseURL := fmt.Sprintf("%s/%s/isos/%s/", definition.Source.URL,
		strings.ToLower(definition.Image.Release),
		definition.Image.ArchitectureMapped)
	s.fname = s.getRelease(definition.Source.URL, definition.Image.Release,
		definition.Source.Variant, definition.Image.ArchitectureMapped)
	if s.fname == "" {
		return fmt.Errorf("Couldn't get name of iso")
	}

	fpath := shared.GetTargetDir(definition.Image)

	// Skip download if raw image exists and has already been decompressed.
	if strings.HasSuffix(s.fname, ".raw.xz") {
		imagePath := filepath.Join(fpath, filepath.Base(strings.TrimSuffix(s.fname, ".xz")))

		stat, err := os.Stat(imagePath)
		if err == nil && stat.Size() > 0 {
			return s.unpackRaw(filepath.Join(fpath, strings.TrimSuffix(s.fname, ".xz")),
				rootfsDir)
		}
	}

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

			if definition.Image.ArchitectureMapped == "armhfp" {
				checksumFile = "sha256sum.txt"
			} else {
				if strings.HasPrefix(definition.Image.Release, "8") {
					checksumFile = "CHECKSUM"
				} else {
					checksumFile = "sha256sum.txt.asc"
				}
			}

			fpath, err := shared.DownloadHash(definition.Image, baseURL+checksumFile, "", nil)
			if err != nil {
				return err
			}

			// Only verify file if possible.
			if strings.HasSuffix(checksumFile, ".asc") {
				valid, err := shared.VerifyFile(filepath.Join(fpath, checksumFile), "",
					definition.Source.Keys, definition.Source.Keyserver)
				if err != nil {
					return err
				}
				if !valid {
					return errors.New("Failed to verify tarball")
				}
			}
		}
	}

	_, err = shared.DownloadHash(definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return errors.Wrap(err, "Error downloading CentOS image")
	}

	if strings.HasSuffix(s.fname, ".raw.xz") || strings.HasSuffix(s.fname, ".raw") {
		return s.unpackRaw(filepath.Join(fpath, s.fname), rootfsDir)
	}

	return s.unpackISO(filepath.Join(fpath, s.fname), rootfsDir)
}

func (s CentOSHTTP) unpackRaw(filePath, rootfsDir string) error {
	roRootDir := filepath.Join(os.TempDir(), "distrobuilder", "rootfs.ro")
	tempRootDir := filepath.Join(os.TempDir(), "distrobuilder", "rootfs")

	os.MkdirAll(roRootDir, 0755)
	defer os.RemoveAll(filepath.Join(os.TempDir(), "distrobuilder"))

	if strings.HasSuffix(filePath, ".raw.xz") {
		// Uncompress raw image
		err := shared.RunCommand("unxz", filePath)
		if err != nil {
			return err
		}
	}

	rawFilePath := strings.TrimSuffix(filePath, ".xz")

	// Figure out the offset
	var buf bytes.Buffer

	err := lxd.RunCommandWithFds(nil, &buf, "fdisk", "-l", "-o", "Start", rawFilePath)
	if err != nil {
		return err
	}

	output := strings.Split(buf.String(), "\n")
	offsetStr := strings.TrimSpace(output[len(output)-2])

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		return err
	}

	// Mount the partition read-only since we don't want to accidently modify it.
	err = shared.RunCommand("mount", "-o", fmt.Sprintf("ro,loop,offset=%d", offset*512),
		rawFilePath, roRootDir)
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
	exitChroot, err := shared.SetupChroot(tempRootDir, shared.DefinitionEnv{}, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to setup chroot")
	}

	err = shared.RunScript(fmt.Sprintf(`#!/bin/sh
set -eux

# Create required files
touch /etc/mtab /etc/fstab

# Create a minimal rootfs
mkdir /rootfs
yum --installroot=/rootfs --disablerepo=* --enablerepo=base -y --releasever=%s install basesystem centos-release yum
rm -rf /rootfs/var/cache/yum

# Disable CentOS kernel repo
sed -ri 's/^enabled=.*/enabled=0/g' /rootfs/etc/yum.repos.d/CentOS-armhfp-kernel.repo
`, s.majorVersion))
	if err != nil {
		exitChroot()
		return err
	}

	exitChroot()

	return shared.RunCommand("rsync", "-qa", tempRootDir+"/rootfs/", rootfsDir)
}

func (s CentOSHTTP) unpackISO(filePath, rootfsDir string) error {
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

# Create required files
touch /etc/mtab /etc/fstab

yum_args=""
mkdir -p /etc/yum.repos.d

if [ -d /mnt/cdrom ]; then
	# Install initial package set
	cd /mnt/cdrom/Packages
	rpm -ivh --nodeps $(ls rpm-*.rpm | head -n1)
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
	yum ${yum_args} -y reinstall yum
else
	if ! [ -f /etc/pki/rpm-gpg/RPM-GPG-KEY-centosofficial ]; then
		mkdir -p /etc/pki/rpm-gpg
		cat <<- "EOF" > /etc/pki/rpm-gpg/RPM-GPG-KEY-centosofficial
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v2.0.22 (GNU/Linux)

mQINBFzMWxkBEADHrskpBgN9OphmhRkc7P/YrsAGSvvl7kfu+e9KAaU6f5MeAVyn
rIoM43syyGkgFyWgjZM8/rur7EMPY2yt+2q/1ZfLVCRn9856JqTIq0XRpDUe4nKQ
8BlA7wDVZoSDxUZkSuTIyExbDf0cpw89Tcf62Mxmi8jh74vRlPy1PgjWL5494b3X
5fxDidH4bqPZyxTBqPrUFuo+EfUVEqiGF94Ppq6ZUvrBGOVo1V1+Ifm9CGEK597c
aevcGc1RFlgxIgN84UpuDjPR9/zSndwJ7XsXYvZ6HXcKGagRKsfYDWGPkA5cOL/e
f+yObOnC43yPUvpggQ4KaNJ6+SMTZOKikM8yciyBwLqwrjo8FlJgkv8Vfag/2UR7
JINbyqHHoLUhQ2m6HXSwK4YjtwidF9EUkaBZWrrskYR3IRZLXlWqeOi/+ezYOW0m
vufrkcvsh+TKlVVnuwmEPjJ8mwUSpsLdfPJo1DHsd8FS03SCKPaXFdD7ePfEjiYk
nHpQaKE01aWVSLUiygn7F7rYemGqV9Vt7tBw5pz0vqSC72a5E3zFzIIuHx6aANry
Gat3aqU3qtBXOrA/dPkX9cWE+UR5wo/A2UdKJZLlGhM2WRJ3ltmGT48V9CeS6N9Y
m4CKdzvg7EWjlTlFrd/8WJ2KoqOE9leDPeXRPncubJfJ6LLIHyG09h9kKQARAQAB
tDpDZW50T1MgKENlbnRPUyBPZmZpY2lhbCBTaWduaW5nIEtleSkgPHNlY3VyaXR5
QGNlbnRvcy5vcmc+iQI3BBMBAgAhBQJczFsZAhsDBgsJCAcDAgYVCAIJCgsDFgIB
Ah4BAheAAAoJEAW1VbOEg8ZdjOsP/2ygSxH9jqffOU9SKyJDlraL2gIutqZ3B8pl
Gy/Qnb9QD1EJVb4ZxOEhcY2W9VJfIpnf3yBuAto7zvKe/G1nxH4Bt6WTJQCkUjcs
N3qPWsx1VslsAEz7bXGiHym6Ay4xF28bQ9XYIokIQXd0T2rD3/lNGxNtORZ2bKjD
vOzYzvh2idUIY1DgGWJ11gtHFIA9CvHcW+SMPEhkcKZJAO51ayFBqTSSpiorVwTq
a0cB+cgmCQOI4/MY+kIvzoexfG7xhkUqe0wxmph9RQQxlTbNQDCdaxSgwbF2T+gw
byaDvkS4xtR6Soj7BKjKAmcnf5fn4C5Or0KLUqMzBtDMbfQQihn62iZJN6ZZ/4dg
q4HTqyVpyuzMXsFpJ9L/FqH2DJ4exGGpBv00ba/Zauy7GsqOc5PnNBsYaHCply0X
407DRx51t9YwYI/ttValuehq9+gRJpOTTKp6AjZn/a5Yt3h6jDgpNfM/EyLFIY9z
V6CXqQQ/8JRvaik/JsGCf+eeLZOw4koIjZGEAg04iuyNTjhx0e/QHEVcYAqNLhXG
rCTTbCn3NSUO9qxEXC+K/1m1kaXoCGA0UWlVGZ1JSifbbMx0yxq/brpEZPUYm+32
o8XfbocBWljFUJ+6aljTvZ3LQLKTSPW7TFO+GXycAOmCGhlXh2tlc6iTc41PACqy
yy+mHmSv
=kkH7
-----END PGP PUBLIC KEY BLOCK-----
EOF
	fi

	cat <<- "EOF" > /etc/yum.repos.d/CentOS-Base.repo
[BaseOS]
name=CentOS-$releasever - Base
mirrorlist=http://mirrorlist.centos.org/?release=$releasever&arch=$basearch&repo=BaseOS&infra=$infra
gpgcheck=1
enabled=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-centosofficial
EOF

	if grep -q "CentOS Stream" /etc/os-release; then
		cat <<- "EOF" > /etc/yum.repos.d/CentOS-Appstream.repo
[AppStream]
name=CentOS-$releasever - Base
mirrorlist=http://mirrorlist.centos.org/?release=$releasever&arch=$basearch&repo=AppStream&infra=$infra
gpgcheck=1
enabled=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-centosofficial
EOF
	fi

	# Use dnf in the boot iso since yum isn't available
	alias yum=dnf
fi

pkgs="basesystem centos-release yum"

if grep -q "CentOS Stream" /etc/os-release; then
	pkgs="${pkgs} centos-stream-repos"
fi

# Create a minimal rootfs
mkdir /rootfs
yum ${yum_args} --installroot=/rootfs -y --releasever=%s --skip-broken install ${pkgs}
rm -rf /rootfs/var/cache/yum
`, gpgKeysPath, s.majorVersion))
	if err != nil {
		exitChroot()
		return err
	}

	exitChroot()

	return shared.RunCommand("rsync", "-qa", tempRootDir+"/rootfs/", rootfsDir)
}

func (s CentOSHTTP) getRelease(URL, release, variant, arch string) string {
	releaseFields := strings.Split(release, ".")

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

	if len(releaseFields) == 3 && !strings.Contains(URL, "vault.centos.org") {
		fmt.Println("Patch releases are only supported when using vault.centos.org as the mirror")
		return ""
	}

	if strings.HasSuffix(releaseFields[0], "-Stream") {
		fields := strings.Split(releaseFields[0], "-")
		// Convert <version>-Stream to Stream-<version>
		releaseFields[0] = fmt.Sprintf("%s-%s", fields[1], fields[0])
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

func (s *CentOSHTTP) getRegexes(arch string, variant string, release string) []*regexp.Regexp {
	releaseFields := strings.Split(release, ".")

	if strings.HasSuffix(releaseFields[0], "-Stream") {
		fields := strings.Split(releaseFields[0], "-")
		// Convert <version>-Stream to Stream-<version>
		releaseFields[0] = fmt.Sprintf("%s-%s", fields[1], fields[0])
	}

	var re []string
	switch len(releaseFields) {
	case 1:
		if arch == "armhfp" {
			re = append(re, fmt.Sprintf("CentOS-Userland-%s-armv7hl-RootFS-(?i:%s)(-\\d+)?-sda.raw.xz",
				releaseFields[0], variant))
		} else {
			re = append(re, fmt.Sprintf("CentOS-%s(.\\d+)*-%s-(?i:%s)(-\\d+)?.iso",
				releaseFields[0], arch, variant))
			re = append(re, fmt.Sprintf("CentOS-%s(.\\d+)*-%s(-\\d+)?-(?i:%s).iso",
				releaseFields[0], arch, variant))
		}
	case 2:
		if arch == "armhfp" {
			re = append(re, fmt.Sprintf("CentOS-Userland-%s.%s-armv7hl-RootFS-(?i:%s)(-\\d+)?-sda.raw.xz",
				releaseFields[0], releaseFields[1], variant))
		} else {
			re = append(re, fmt.Sprintf("CentOS-%s.%s-%s-(?i:%s)(-\\d+)?.iso",
				releaseFields[0], releaseFields[1], arch, variant))
			re = append(re, fmt.Sprintf("CentOS-%s-%s-%s-(?i:%s).iso",
				releaseFields[0], arch, releaseFields[1], variant))
		}
	case 3:
		if arch == "x86_64" {
			re = append(re, fmt.Sprintf("CentOS-%s.%s-%s-%s-(?i:%s)(-\\d+)?.iso",
				releaseFields[0], releaseFields[1], releaseFields[2], arch, variant))

			if len(releaseFields[1]) == 1 {
				re = append(re, fmt.Sprintf("CentOS-%s-%s-(?i:%s)-%s-0%s.iso",
					releaseFields[0], arch, variant, releaseFields[2], releaseFields[1]))
			} else {
				re = append(re, fmt.Sprintf("CentOS-%s-%s-(?i:%s)-%s-%s.iso",
					releaseFields[0], arch, variant, releaseFields[2], releaseFields[1]))
			}

			re = append(re, fmt.Sprintf("CentOS-%s-%s-(?i:%s)-%s.iso",
				releaseFields[0], arch, variant, releaseFields[2]))
			re = append(re, fmt.Sprintf("CentOS-%s-%s-%s-(?i:%s).iso",
				releaseFields[0], arch, releaseFields[2], variant))
		}
	}

	regexes := make([]*regexp.Regexp, len(re))

	for i, r := range re {
		regexes[i] = regexp.MustCompile(r)
	}

	return regexes
}

func (s CentOSHTTP) unpackRootfsImage(imageFile string, target string) error {
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
