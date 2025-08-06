package sources

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lxc/distrobuilder/shared"
)

type almalinux struct {
	commonRHEL

	fname        string
	majorVersion string
}

// Run downloads the tarball and unpacks it.
func (s *almalinux) Run() error {
	var err error

	s.majorVersion = strings.Split(s.definition.Image.Release, ".")[0]

	baseURL := fmt.Sprintf("%s/%s/isos/%s/", s.definition.Source.URL,
		strings.ToLower(s.definition.Image.Release),
		s.definition.Image.ArchitectureMapped)
	s.fname, err = s.getRelease(s.definition.Source.URL, s.definition.Image.Release,
		s.definition.Source.Variant, s.definition.Image.ArchitectureMapped)
	if err != nil {
		return fmt.Errorf("Failed to get release: %w", err)
	}

	fpath := s.getTargetDir()

	// Skip download if raw image exists and has already been decompressed.
	if strings.HasSuffix(s.fname, ".raw.xz") {
		imagePath := filepath.Join(fpath, filepath.Base(strings.TrimSuffix(s.fname, ".xz")))

		stat, err := os.Stat(imagePath)
		if err == nil && stat.Size() > 0 {
			s.logger.WithField("file", filepath.Join(fpath, strings.TrimSuffix(s.fname, ".xz"))).Info("Unpacking raw image")

			return s.unpackRaw(filepath.Join(fpath, strings.TrimSuffix(s.fname, ".xz")),
				s.rootfsDir, s.rawRunner)
		}
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("Failed to parse URL %q: %w", baseURL, err)
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

			fpath, err := s.DownloadHash(s.definition.Image, baseURL+checksumFile, "", nil)
			if err != nil {
				return fmt.Errorf("Failed to download %q: %w", baseURL+checksumFile, err)
			}

			// Only verify file if possible.
			if strings.HasSuffix(checksumFile, ".asc") {
				valid, err := s.VerifyFile(filepath.Join(fpath, checksumFile), "")
				if err != nil {
					return fmt.Errorf("Failed to verify %q: %w", checksumFile, err)
				}

				if !valid {
					return fmt.Errorf("Invalid signature for %q", filepath.Join(fpath, checksumFile))
				}
			}
		}
	}

	_, err = s.DownloadHash(s.definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", baseURL+s.fname, err)
	}

	if strings.HasSuffix(s.fname, ".raw.xz") || strings.HasSuffix(s.fname, ".raw") {
		s.logger.WithField("file", filepath.Join(fpath, s.fname)).Info("Unpacking raw image")

		return s.unpackRaw(filepath.Join(fpath, s.fname), s.rootfsDir, s.rawRunner)
	}

	s.logger.WithField("file", filepath.Join(fpath, s.fname)).Info("Unpacking ISO")

	return s.unpackISO(filepath.Join(fpath, s.fname), s.rootfsDir, s.isoRunner)
}

func (s *almalinux) rawRunner() error {
	err := shared.RunScript(s.ctx, fmt.Sprintf(`#!/bin/sh
	set -eux

	# Create required files
	touch /etc/mtab /etc/fstab

	# Create a minimal rootfs
	mkdir /rootfs
	yum --installroot=/rootfs --disablerepo=* --enablerepo=base -y --releasever=%s install basesystem almalinux-release yum
	rm -rf /rootfs/var/cache/yum
	`, s.majorVersion))
	if err != nil {
		return fmt.Errorf("Failed to run script: %w", err)
	}

	return nil
}

func (s *almalinux) isoRunner(gpgKeysPath string) error {
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
	if ! [ -f /etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux ]; then
		mv /etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-* /etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux
	fi

	cat <<- "EOF" > /etc/yum.repos.d/almalinux.repo
	[baseos]
	name=AlmaLinux $releasever - BaseOS
	baseurl=https://repo.almalinux.org/almalinux/$releasever/BaseOS/$basearch/os/
	gpgcheck=1
	enabled=1
	gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux
	EOF

	if [ "${RELEASE}" -eq 10 ]; then
		# Disable gpg checks in the boot iso since rpmkeys isn't available
		yum_args="${yum_args} --nogpgcheck"
	fi

	# Use dnf in the boot iso since yum isn't available
	alias yum=dnf
fi

pkgs="basesystem almalinux-release yum"

# Create a minimal rootfs
mkdir /rootfs
yum ${yum_args} --installroot=/rootfs -y --releasever=%s --skip-broken install ${pkgs}
rm -rf /rootfs/var/cache/yum
`, gpgKeysPath, s.majorVersion))
	if err != nil {
		return fmt.Errorf("Failed to run script: %w", err)
	}

	return nil
}

func (s *almalinux) getRelease(URL, release, variant, arch string) (string, error) {
	fullURL := URL + path.Join("/", strings.ToLower(release), "isos", arch)

	var (
		err  error
		resp *http.Response
	)

	err = shared.Retry(func() error {
		resp, err = http.Get(fullURL)
		if err != nil {
			return fmt.Errorf("Failed to GET %q: %w", fullURL, err)
		}

		return nil
	}, 3)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to read body: %w", err)
	}

	re := s.getRegexes(arch, variant, release)

	for _, r := range re {
		matches := r.FindAllString(string(body), -1)
		if len(matches) > 0 {
			return matches[len(matches)-1], nil
		}
	}

	return "", nil
}

func (s *almalinux) getRegexes(arch string, variant string, release string) []*regexp.Regexp {
	releaseFields := strings.Split(release, ".")

	var re []string
	switch len(releaseFields) {
	case 1:
		re = append(re, fmt.Sprintf("AlmaLinux-%s(\\.\\d+)*-%s-(?i:%s)(-\\d+)?.iso",
			releaseFields[0], arch, variant))
		re = append(re, fmt.Sprintf("AlmaLinux-%s(.\\d+)*-(beta|rc)-\\d-%s-(?i:%s).iso",
			releaseFields[0], arch, variant))
	case 2:
		re = append(re, fmt.Sprintf("AlmaLinux-%s\\.%s-%s-(?i:%s)(-\\d+)?.iso",
			releaseFields[0], releaseFields[1], arch, variant))
		re = append(re, fmt.Sprintf("AlmaLinux-%s\\.%s-(beta|rc)-\\d-%s-(?i:%s).iso",
			releaseFields[0], releaseFields[1], arch, variant))
	}

	regexes := make([]*regexp.Regexp, len(re))

	for i, r := range re {
		regexes[i] = regexp.MustCompile(r)
	}

	return regexes
}
