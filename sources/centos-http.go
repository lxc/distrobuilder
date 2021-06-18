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

	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
)

type centOS struct {
	commonRHEL

	fname        string
	majorVersion string
}

func (s *centOS) Run() error {
	if strings.HasSuffix(s.definition.Image.Release, "-Stream") {
		s.majorVersion = strings.ToLower(s.definition.Image.Release)
	} else {
		s.majorVersion = strings.Split(s.definition.Image.Release, ".")[0]
	}

	baseURL := fmt.Sprintf("%s/%s/isos/%s/", s.definition.Source.URL,
		strings.ToLower(s.definition.Image.Release),
		s.definition.Image.ArchitectureMapped)
	s.fname = s.getRelease(s.definition.Source.URL, s.definition.Image.Release,
		s.definition.Source.Variant, s.definition.Image.ArchitectureMapped)
	if s.fname == "" {
		return fmt.Errorf("Couldn't get name of iso")
	}

	fpath := shared.GetTargetDir(s.definition.Image)

	// Skip download if raw image exists and has already been decompressed.
	if strings.HasSuffix(s.fname, ".raw.xz") {
		imagePath := filepath.Join(fpath, filepath.Base(strings.TrimSuffix(s.fname, ".xz")))

		stat, err := os.Stat(imagePath)
		if err == nil && stat.Size() > 0 {
			return s.unpackRaw(filepath.Join(fpath, strings.TrimSuffix(s.fname, ".xz")),
				s.rootfsDir, s.rawRunner)
		}
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	checksumFile := ""
	if !s.definition.Source.SkipVerification {
		// Force gpg checks when using http
		if url.Scheme != "https" {
			if len(s.definition.Source.Keys) == 0 {
				return errors.New("GPG keys are required if downloading from HTTP")
			}

			if s.definition.Image.ArchitectureMapped == "armhfp" {
				checksumFile = "sha256sum.txt"
			} else {
				if strings.HasPrefix(s.definition.Image.Release, "8") {
					checksumFile = "CHECKSUM"
				} else {
					checksumFile = "sha256sum.txt.asc"
				}
			}

			fpath, err := shared.DownloadHash(s.definition.Image, baseURL+checksumFile, "", nil)
			if err != nil {
				return err
			}

			// Only verify file if possible.
			if strings.HasSuffix(checksumFile, ".asc") {
				valid, err := shared.VerifyFile(filepath.Join(fpath, checksumFile), "",
					s.definition.Source.Keys, s.definition.Source.Keyserver)
				if err != nil {
					return err
				}
				if !valid {
					return errors.New("Failed to verify tarball")
				}
			}
		}
	}

	_, err = shared.DownloadHash(s.definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return errors.Wrap(err, "Error downloading CentOS image")
	}

	if strings.HasSuffix(s.fname, ".raw.xz") || strings.HasSuffix(s.fname, ".raw") {
		return s.unpackRaw(filepath.Join(fpath, s.fname), s.rootfsDir, s.rawRunner)
	}

	return s.unpackISO(filepath.Join(fpath, s.fname), s.rootfsDir, s.isoRunner)
}

func (s *centOS) rawRunner() error {
	err := shared.RunScript(fmt.Sprintf(`#!/bin/sh
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
		return err
	}

	return nil
}

func (s *centOS) isoRunner(gpgKeysPath string) error {
	err := shared.RunScript(fmt.Sprintf(`#!/bin/sh
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
		return err
	}

	return nil
}

func (s *centOS) getRelease(URL, release, variant, arch string) string {
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

func (s *centOS) getRegexes(arch string, variant string, release string) []*regexp.Regexp {
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
			re = append(re, fmt.Sprintf("CentOS-Userland-%s-armv7hl-(RootFS|generic)-(?i:%s)(-\\d+)?-sda.raw.xz",
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
