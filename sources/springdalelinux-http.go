package sources

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/shared"
)

type springdalelinux struct {
	commonRHEL

	fname        string
	majorVersion string
}

// Run downloads the tarball and unpacks it.
func (s *springdalelinux) Run() error {
	s.majorVersion = strings.Split(s.definition.Image.Release, ".")[0]

	// Example: http://puias.princeton.edu/data/puias/8.3/x86_64/os/images/boot.iso
	baseURL := fmt.Sprintf("%s/%s/%s/os/images/", s.definition.Source.URL,
		strings.ToLower(s.definition.Image.Release),
		s.definition.Image.ArchitectureMapped)
	s.fname = "boot.iso"

	fpath := s.getTargetDir()

	_, err := s.DownloadHash(s.definition.Image, baseURL+s.fname, "", nil)
	if err != nil {
		return fmt.Errorf("Error downloading %q: %w", baseURL+s.fname, err)
	}

	s.logger.WithField("file", filepath.Join(fpath, s.fname)).Info("Unpacking ISO")

	err = s.unpackISO(filepath.Join(fpath, s.fname), s.rootfsDir, s.isoRunner)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", filepath.Join(fpath, s.fname), err)
	}

	return nil
}

func (s *springdalelinux) isoRunner(gpgKeysPath string) error {
	err := shared.RunScript(s.ctx, fmt.Sprintf(`#!/bin/sh
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
	. /etc/os-release
	mkdir -p /etc/pki/rpm-gpg
	if [ "${VERSION_ID:0:1}" -eq "9" ]; then
		cat <<- "EOF" > /etc/pki/rpm-gpg/RPM-GPG-KEY-springdale
-----BEGIN PGP PUBLIC KEY BLOCK-----

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
GrticHp1XlAnezB5yw==
=1nA4
-----END PGP PUBLIC KEY BLOCK-----
EOF
	elif ! [ -f /etc/pki/rpm-gpg/RPM-GPG-KEY-springdale ]; then
		cat <<- "EOF" > /etc/pki/rpm-gpg/RPM-GPG-KEY-springdale
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v2.0.14 (GNU/Linux)

mQILBEzhkmYBEAD1+atknNh1ufb0QZNKl6uMnOnAL2bk8v6VmUjVTNTJ6JBXdREX
omVRlEDv1Jw9/11tDEI0JzZieqogahJ3GDqENXhLlfpgev+YHES6jrH8XEYkin/T
wxuUyKvqoAk60wqC6Uv/TkHpVOYP9WHtNbybl3Pu8f2tzqdrE20HmQgEzVjTyHEw
33WeAXWlhb/rIMcJ/edk0/hQ0Yu1NR34g/R/1azfz4uyx0qtrTDvaS7qxX5ZSUYZ
tHzS8JjFpzRHI3A++Jv6yXMdzekn54TYob+DeKoRsdtfI0KryOv+92tcw4yM35CK
d4apq8t35v56wsRXhwVZWEYn3BfIsdDoCF4XXH8nx0B1KJii6uYo1iiyIU10cT+g
YCPVIzT+B/FqAmltx3VJg87sZ6QVFp3oFtIOlu14NQBv3wqUneCNK88EZAxi/kqy
kFLq4NfWpn2dVZYRlJddWTXZ+qqaYQ6aI9HyGx5/QTI1hsxcOykrm8woZvptYeRQ
IfkDFPgqTmdTqFaL5TQc43FvOLss9kBJ2FSbi3WpRIJW0/AQGc9/97SUnC0T075O
R3cK+dQvI3QV3/p1UEowXlSHDFc5CrqKT11zMG9KE4UGm08yDbz0d5IbH0Cpzuku
vidjBlsaQ/+OldjSFjHhCFLz55lLya6eMpPaScEKqoWNYzuNv9kDgVWXNwAGKbRN
UFVJQVMgTGludXggRGlzdHJpYnV0aW9uIChSUE0gSW50ZWdyaXR5IFNpZ25hdHVy
ZSkgPHB1aWFzQG1hdGgucHJpbmNldG9uLmVkdT6JAjQEEwECAB4FAkzhkmYCGwMG
CwkIBwMCAxUCAwMWAgECHgECF4AACgkQFs/DM0GkCUi22xAAtoeFPRpYAoaq6+Ru
nRX5GCDQl6DlOtVxLVclNZzGpnw8Extid+AOLqDXcfncyf04YhlEHj4misz/rDCI
a5bRWoNjPHAgHpzCX7+I6pNr1hY9SOW94BEdng9IGGK0XhFBzflmySLZEC9E2ZYe
RgWKJcDbyM9sDc2g440ICkn8DOWTvKMcQ7f0AzYtARXfmAEMqgqzNV+0wDJmdEHY
7rif51U8bCOKns/UFKSA3WqUKhn5v2xo4OVqkm+bVG3z04KRAIUWIZIK8RHEp6wk
clls8afYSufJmmUeczbE/wDqEgMSE3qGlcQRxTO3EMORb3nwWo5QAA4I/QPFrFoC
QZQbaLNOx8P7dnfDoarJrUPYBBUFmMKvHUnSwv696QZhz70RvgjTHcbSyrnmE76C
/XU0zUpeWN6FEb77zA1pIlqVf4hqRs+PCaG2sytBQVYEpYgGnoUPSWIT4a6NJtn+
WwJHOqRYrGGTM0Z6V7IgMAkqiwEECn5eDUXYhqwUsyuVkbeOBWTc6nhsPIH3QC86
sL0X/1hztP5sDoCne6SY3X3IyglvApvsKn0TOcVCvbYNhbg2bfPdvfmtSAV3/iMU
yPw2JgfcvLeF1tiMQ7i5PfgyOn0Y3/lZjcclYHa1P5PEoTCA2lU7jLm0lmgIrh4N
vYD0DGRZTGNEJYUDZFgRyynIOxk=
=mKoc
-----END PGP PUBLIC KEY BLOCK-----
EOF
	fi

	if [[ ${VERSION_ID:0:1} == "7" ]]
	then
		fname="$(curl -s http://springdale.princeton.edu/data/springdale/7/x86_64/os/Packages/ | grep -Eo 'yum-[[:digit:]].+\.noarch\.rpm')"

		if [ -n "${fname}" ]; then
			wget http://springdale.princeton.edu/data/springdale/7/x86_64/os/Packages/"${fname}"
			rpm -ivh --nodeps yum*.rpm
		fi
	fi


	if [[ ${VERSION_ID:0:1} == "8" || ${VERSION_ID:0:1} == "9" ]]
	then
	cat <<- "EOF" > /etc/yum.repos.d/Springdale-Base.repo
[sdl8-baseos]
name=Springdale core Base $releasever - $basearch
mirrorlist=http://springdale.princeton.edu/data/springdale/$releasever/$basearch/os/BaseOS/mirrorlist
#baseurl=http://springdale.princeton.edu/data/springdale/$releasever/$basearch/os/BaseOS
gpgcheck=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-springdale

[sdl8-baseos-updates]
name=Springdale core Updates $releasever - $basearch
mirrorlist=http://springdale.princeton.edu/data/springdale/updates/$releasever/BaseOS/$basearch/mirrorlist
#baseurl=http://springdale.princeton.edu/data/springdale/updates/$releasever/BaseOS/$basearch
gpgcheck=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-springdale
EOF
	# Use dnf in the boot iso since yum isn't available
	alias yum=dnf
	fi

	if [[ ${VERSION_ID:0:1} == "7" ]]
	then
	cat <<- "EOF" > /etc/yum.repos.d/Springdale-Base.repo
[core]
name=Springdale core Base $releasever - $basearch
#baseurl=file:///springdale/$releasever/$basearch/os
#mirrorlist=http://mirror.math.princeton.edu/pub/springdale/puias/$releasever/$basearch/os/mirrorlist
baseurl=http://springdale.princeton.edu/data/springdale/$releasever/$basearch/os
gpgcheck=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-springdale

[updates]
name=Springdale core Updates $releasever - $basearch
#baseurl=file:///springdale/updates/$releasever/en/os/$basearch
#mirrorlist=http://mirror.math.princeton.edu/pub/springdale/puias/updates/$releasever/en/os/$basearch/mirrorlist
baseurl=http://springdale.princeton.edu/data/springdale/updates/$releasever/en/os/$basearch
gpgcheck=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-springdale
EOF
	fi
fi

pkgs="basesystem springdale-release yum"

# Create a minimal rootfs
mkdir /rootfs
echo "install rootfs"
yum ${yum_args} --installroot=/rootfs -y --releasever=%s --skip-broken install ${pkgs}
rm -rf /rootfs/var/cache/yum
`, gpgKeysPath, s.majorVersion))
	if err != nil {
		return fmt.Errorf("Failed to run ISO script: %w", err)
	}

	return nil
}
