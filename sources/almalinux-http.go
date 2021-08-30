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
		return errors.WithMessage(err, "Failed to get release")
	}

	fpath := shared.GetTargetDir(s.definition.Image)

	// Skip download if raw image exists and has already been decompressed.
	if strings.HasSuffix(s.fname, ".raw.xz") {
		imagePath := filepath.Join(fpath, filepath.Base(strings.TrimSuffix(s.fname, ".xz")))

		stat, err := os.Stat(imagePath)
		if err == nil && stat.Size() > 0 {
			s.logger.Infow("Unpacking raw image", "file", filepath.Join(fpath, strings.TrimSuffix(s.fname, ".xz")))

			return s.unpackRaw(filepath.Join(fpath, strings.TrimSuffix(s.fname, ".xz")),
				s.rootfsDir, s.rawRunner)
		}
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return errors.WithMessagef(err, "Failed to parse URL %q", baseURL)
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
				return errors.WithMessagef(err, "Failed to download %q", baseURL+checksumFile)
			}

			// Only verify file if possible.
			if strings.HasSuffix(checksumFile, ".asc") {
				valid, err := shared.VerifyFile(filepath.Join(fpath, checksumFile), "",
					s.definition.Source.Keys, s.definition.Source.Keyserver)
				if err != nil {
					return errors.WithMessagef(err, "Failed to verify %q", checksumFile)
				}
				if !valid {
					return errors.Errorf("Invalid signature for %q", filepath.Join(fpath, checksumFile))
				}
			}
		}
	}

	_, err = shared.DownloadHash(s.definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return errors.WithMessagef(err, "Failed to download %q", baseURL+s.fname)
	}

	if strings.HasSuffix(s.fname, ".raw.xz") || strings.HasSuffix(s.fname, ".raw") {
		s.logger.Infow("Unpacking raw image", "file", filepath.Join(fpath, s.fname))

		return s.unpackRaw(filepath.Join(fpath, s.fname), s.rootfsDir, s.rawRunner)
	}

	s.logger.Infow("Unpacking ISO", "file", filepath.Join(fpath, s.fname))

	return s.unpackISO(filepath.Join(fpath, s.fname), s.rootfsDir, s.isoRunner)
}

func (s *almalinux) rawRunner() error {
	err := shared.RunScript(fmt.Sprintf(`#!/bin/sh
	set -eux

	# Create required files
	touch /etc/mtab /etc/fstab

	# Create a minimal rootfs
	mkdir /rootfs
	yum --installroot=/rootfs --disablerepo=* --enablerepo=base -y --releasever=%s install basesystem almalinux-release yum
	rm -rf /rootfs/var/cache/yum
	`, s.majorVersion))
	if err != nil {
		return errors.WithMessage(err, "Failed to run script")
	}

	return nil
}

func (s *almalinux) isoRunner(gpgKeysPath string) error {
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
if ! [ -f /etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux ]; then
	mkdir -p /etc/pki/rpm-gpg
	cat <<- "EOF" > /etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v1

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
72EHtteVBoVn0eg6
=rEWJ
-----END PGP PUBLIC KEY BLOCK-----
EOF
fi

cat <<- "EOF" > /etc/yum.repos.d/almalinux.repo
[baseos]
name=AlmaLinux $releasever - BaseOS
baseurl=https://repo.almalinux.org/almalinux/$releasever/BaseOS/$basearch/os/
gpgcheck=1
enabled=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux
EOF

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
		return errors.WithMessage(err, "Failed to run script")
	}

	return nil
}

func (s *almalinux) getRelease(URL, release, variant, arch string) (string, error) {
	fullURL := URL + path.Join("/", strings.ToLower(release), "isos", arch)

	resp, err := http.Get(fullURL)
	if err != nil {
		return "", errors.WithMessagef(err, "Failed to GET %q", fullURL)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.WithMessage(err, "Failed to read body")
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
