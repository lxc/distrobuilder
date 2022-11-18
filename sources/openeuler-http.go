package sources

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/lxc/distrobuilder/shared"
)

type openEuler struct {
	commonRHEL
	fileName     string
	checksumFile string
}

const (
	isoFileName = "%s-%s-dvd.iso"
	shaFileName = "%s-%s-dvd.iso.sha256sum"
)

func (s *openEuler) Run() error {
	var err error
	baseURL := fmt.Sprintf("%s/%s/ISO/%s/", s.definition.Source.URL,
		s.definition.Image.Release,
		s.definition.Image.Architecture)

	fpath := s.getTargetDir()

	s.fileName = fmt.Sprintf(isoFileName, s.definition.Image.Name, s.definition.Image.Architecture)
	s.checksumFile = fmt.Sprintf(shaFileName, s.definition.Image.Name, s.definition.Image.Architecture)

	_, err = url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("Failed to parse URL %s: %w", baseURL, err)
	}

	_, err = s.DownloadHash(s.definition.Image, baseURL+s.fileName, baseURL+s.checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %s: %w", baseURL+s.fileName, err)
	}

	source := filepath.Join(fpath, s.fileName)

	s.logger.Info("Unpacking image folder", "rootfsDir", s.rootfsDir, "cacheDir", s.cacheDir)

	err = s.unpackISO(source, s.rootfsDir, s.isoRunner)
	if err != nil {
		return fmt.Errorf("Failed to unpack %s: %w", source, err)
	}
	return nil
}

func (s *openEuler) isoRunner(gpgKeysPath string) error {
	err := shared.RunScript(s.ctx, fmt.Sprintf(`#!/bin/sh
set -eux

GPG_KEYS="%s"

# Create required files
touch /etc/mtab /etc/fstab

yum_args=""
mkdir -p /etc/yum.repos.d

if which dnf; then
	alias yum=dnf
else
	# for openEuler packageDir and repoDir always exist.
	# Install initial package set
	cd /mnt/cdrom/Packages
	rpm -ivh --nodeps $(ls rpm-*.rpm | head -n1)
	rpm -ivh --nodeps $(ls yum-*.rpm | head -n1)
fi

# Add cdrom repo
cat <<- EOF > /etc/yum.repos.d/cdrom.repo
[cdrom]
name=Install CD-ROM
baseurl=file:///mnt/cdrom
enabled=0
EOF

gpg_keys_official="file:///etc/pki/rpm-gpg/RPM-GPG-KEY-openEuler"

if [ -n "${GPG_KEYS}" ]; then
	echo gpgcheck=1 >> /etc/yum.repos.d/cdrom.repo
	echo gpgkey=${gpg_keys_official} ${GPG_KEYS} >> /etc/yum.repos.d/cdrom.repo
else
	echo gpgcheck=0 >> /etc/yum.repos.d/cdrom.repo
fi

yum_args="--disablerepo=* --enablerepo=cdrom"

# newest install.img doesnt have rpm installed,
# so install rpm firstly
if [ -z "$(which rpmkeys)" ]; then
	cd /mnt/cdrom/Packages
	yum ${yum_args} -y install rpm --nogpgcheck
fi

yum ${yum_args} -y install yum dnf

pkgs="basesystem openEuler-release yum"

# Create a minimal rootfs
mkdir /rootfs
yum ${yum_args} --installroot=/rootfs -y  --skip-broken install ${pkgs}
rm -rf /rootfs/var/cache/yum
rm -rf /etc/yum.repos.d/cdrom.repo
# Remove all files in mnt packages
rm -rf /mnt/cdrom
`, gpgKeysPath))
	if err != nil {
		return fmt.Errorf("Failed to run script: %w", err)
	}

	return nil
}
