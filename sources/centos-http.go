package sources

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

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

	var err error

	baseURL := fmt.Sprintf("%s/%s/isos/%s/", s.definition.Source.URL,
		strings.ToLower(s.definition.Image.Release),
		s.definition.Image.ArchitectureMapped)
	s.fname, err = s.getRelease(s.definition.Source.URL, s.definition.Image.Release,
		s.definition.Source.Variant, s.definition.Image.ArchitectureMapped)
	if err != nil {
		return fmt.Errorf("Failed to get release: %w", err)
	}

	fpath := shared.GetTargetDir(s.definition.Image)

	// Skip download if raw image exists and has already been decompressed.
	if strings.HasSuffix(s.fname, ".raw.xz") {
		imagePath := filepath.Join(fpath, filepath.Base(strings.TrimSuffix(s.fname, ".xz")))

		stat, err := os.Stat(imagePath)
		if err == nil && stat.Size() > 0 {
			tarball := filepath.Join(fpath, strings.TrimSuffix(s.fname, ".xz"))

			err = s.unpackRaw(tarball, s.rootfsDir, s.rawRunner)
			if err != nil {
				return fmt.Errorf("Failed to unpack %q: %w", tarball, err)
			}

			return nil
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

			fpath, err := shared.DownloadHash(s.definition.Image, baseURL+checksumFile, "", nil)
			if err != nil {
				return fmt.Errorf("Failed to download %q: %w", baseURL+checksumFile, err)
			}

			// Only verify file if possible.
			if strings.HasSuffix(checksumFile, ".asc") {
				valid, err := shared.VerifyFile(filepath.Join(fpath, checksumFile), "",
					s.definition.Source.Keys, s.definition.Source.Keyserver)
				if err != nil {
					return fmt.Errorf("Failed to verify %q: %w", checksumFile, err)
				}
				if !valid {
					return fmt.Errorf("Invalid signature for %q", checksumFile)
				}
			}
		}
	}

	_, err = shared.DownloadHash(s.definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", baseURL+s.fname, err)
	}

	source := filepath.Join(fpath, s.fname)

	s.logger.Infow("Unpacking image", "file", source)

	if strings.HasSuffix(s.fname, ".raw.xz") || strings.HasSuffix(s.fname, ".raw") {
		err = s.unpackRaw(source, s.rootfsDir, s.rawRunner)
	} else {
		err = s.unpackISO(source, s.rootfsDir, s.isoRunner)
	}
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", source, err)
	}

	return nil
}

func (s *centOS) rawRunner() error {
	err := shared.RunScript(fmt.Sprintf(`#!/bin/sh
set -eux

version="%s"

# Create required files
touch /etc/mtab /etc/fstab

# Create a minimal rootfs
mkdir /rootfs

if [ "${version}" = 7 ]; then
	repo="base"
else
	repo="BaseOS"
fi

yum --installroot=/rootfs --disablerepo=* --enablerepo=${repo} -y --releasever=${version} install basesystem centos-release yum
rm -rf /rootfs/var/cache/yum

# Disable CentOS kernel repo
if [ -e /rootfs/etc/yum.repos.d/CentOS-armhfp-kernel.repo ]; then
	sed -ri 's/^enabled=.*/enabled=0/g' /rootfs/etc/yum.repos.d/CentOS-armhfp-kernel.repo
fi
`, s.majorVersion))
	if err != nil {
		return fmt.Errorf("Failed to run script: %w", err)
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
mkdir -p /etc/pki/rpm-gpg

# Add GPG keys for aarch64, Arm32, and ppc64
cat <<-"EOF" >/etc/pki/rpm-gpg/RPM-GPG-KEY-CentOS-7-aarch64
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v2.0.22 (GNU/Linux)

mQENBFW3h2gBCADPM3WDbdHNnMAB0FPvVRIBjGpkpeWT5rsbMZbk35tCx7MbAhMk
zcN519xw7DGVLigFd68S3W2Lrde6ioyVQ1SVSJ7z84U4uYUfSa858Dskxxy021Ip
NrocTrziy773v1gCPwA5xeT89bgzsMVMzCSy0U7TeqMDhN2urEMG5CCEpy0K9XZv
bpUexhn7TbP10g5BzC9igd498QcW/69Oz5OK7WcZOtqmGn78pGBCH2ly+IqIV6ZS
9yXC6jOmOnA8fM0gKJAelhQALd77cULMSGbu96ReG3BEFlgWQjbtZG3L5BvMVInw
MkUQEntHvjp6oHtPiIAc3VtLq0IxWVygFHNRABEBAAG0cENlbnRPUyBBbHRBcmNo
IFNJRyAtIEFBcmNoNjQgKGh0dHA6Ly93aWtpLmNlbnRvcy5vcmcvU3BlY2lhbElu
dGVyZXN0R3JvdXAvQWx0QXJjaC9BQXJjaDY0KSA8c2VjdXJpdHlAY2VudG9zLm9y
Zz6JATkEEwECACMFAlW3h2gCGwMHCwkIBwMCAQYVCAIJCgsEFgIDAQIeAQIXgAAK
CRBsfLbvMF1J1pSFCACQbLvjwCFdgr0DpVJZ0o50Dcl8jYzZtd/NZOBNYXi/TQza
c6DFhiAj72zkgOGb+xznUXJJIiOLCgyJBUdJQSRx/EfVb9ftd4kSOA/wErOhDV71
Hyww9M/gz82SjHF9qq8ofDto6ZfJMfiLX4aZwR39jZzS5Gm+bH5FfgxlwG0V88fu
aKlzsn3p975uD659tSKae4xLysxkBG6oDaXvnWI2/UGC724gN+R3aKe9kI0wk8wA
h5Qzf7+jRk0qb859rryno1rBpuzxJcwg5qvN2PXG3xDFOHG+3LX3mV3UnVAqCjHO
zyGnzAAiNfBwgMyu6bu4lXd4hbZKy73RwnouQkuA
=qiwp
-----END PGP PUBLIC KEY BLOCK-----
EOF

cat <<-"EOF" >/etc/pki/rpm-gpg/RPM-GPG-KEY-CentOS-SIG-AltArch-Arm32
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v2.0.22 (GNU/Linux)

mQENBFZYUi8BCADtrzJRu2Q3WGb5ExbmAB8CGWDAbVOTLZBA0bSj+i63LsUDdHkU
sKpOGEaRPhagB27lkVUMOkcOIodYAbQZDbF788KDxeF4BopORbGXdo14OMEmoVq6
rWPDoYs7Zv7G8blQa0IBE/BqdjYxyXZ0CSt+OLQ8r3G8ZB//SbZSTWWJcp2aN5oE
79yB+tEfYznGzETZY8gzBOcKIk/ifYVNHHS65ldgOd3KQK7/vjWVc9LDOLcFcwXj
YABSaUTsc3SkYKQ71SuxLssBWxSGaiZWBdN7s0FZFMDagWtKW1jQDlIhoRSULfpL
m5Y306pEqNOdiNgAnipXPL4NzWv0zFVHoWaFABEBAAG0bUNlbnRPUyBBbHRBcmNo
IFNJRyAtIEFybTMyIChodHRwczovL3dpa2kuY2VudG9zLm9yZy9TcGVjaWFsSW50
ZXJlc3RHcm91cC9BbHRBcmNoL0FybTMyKSA8c2VjdXJpdHlAY2VudG9zLm9yZz6J
ATkEEwECACMFAlZYUi8CGwMHCwkIBwMCAQYVCAIJCgsEFgIDAQIeAQIXgAAKCRDK
/vEbYlBf5qvhB/9R8GXKz71u66U1VTvlDEh4tz7LzKNUBAtEH9fvox1Y8Mh1+VKK
h7WtAWXsAkBvy7HeJ/GCUgvbgBjc7qpVjq/dipUTt+c51TLkoSa0msv4aJnA5azU
7+9qD/qvnjEZVgstFGyTQ+m5v9N3KdAWyw2Xi1V820bmmj+vlVzGFbQo2UPps+7d
bXZ9xI9Lmme/KD4tctjg9lnoCXmFIHGZfMVCoCyk42+p5EHlSZhYIRyIIhjpELlL
gllMZz1Bdp+V51zndIm7Fe1d6jcSEjpPjRecIxfr5PBLAu3j/VbjBK90u8AKSKY9
q5eFcyxxA1r2IdmItGVwz73gSz8WkJoh8QeN
=72OZ
-----END PGP PUBLIC KEY BLOCK-----
EOF

cat <<-"EOF" >/etc/pki/rpm-gpg/RPM-GPG-KEY-CentOS-SIG-AltArch-7-ppc64
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v2.0.22 (GNU/Linux)

mQENBFZYUWkBCADomwJs4B6eBhhHmkBxaTQBNg2SicdZZWfb9+VArLqZ+Qyez3YQ
V1Bq2dBaDv2HIpTI8AHyT/KL/VuF1cdmGK8Q+uhqVxbFIP3giuaNHdV+DLx7suid
aKP0MA/1fs5x4RDvRmHVm0bPRwUWK84aWyh2Ux1D9I8HWsmDamAVKUinocnWWG0K
sNsV2uTuHeXYrJB0lex1nD1ColEa4CjmRxHMFYhoaFfw+mUUJ6rrN+zPdettxzbe
HPBVhNWpfOcQdEIrPWwhMCJJYOnPQ7OpZBZ7088Bc7JVA4RHMo54MuuU2t1Th71H
l7hcF9ueIKXqnsoAWFoG+p4UOy+OHU11THp3ABEBAAG0aUNlbnRPUyBBbHRBcmNo
IFNJRyAtIFBvd2VyUEMgKGh0dHBzOi8vd2lraS5jZW50b3Mub3JnL1NwZWNpYWxJ
bnRlcmVzdEdyb3VwL0FsdEFyY2gpIDxzZWN1cml0eUBjZW50b3Mub3JnPokBOQQT
AQIAIwUCVlhRaQIbAwcLCQgHAwIBBhUIAgkKCwQWAgMBAh4BAheAAAoJEKlju9v1
M/T6HPsH/jLoRihPGZrdNjnVRSx/7hzQ+csdpgwRYSgJOeLTJAmemXYxiAQ0Wh+Z
AiDA6hdUu973Y/aTZbOoX+trb6SaEquGLLxhFgC21whVYfRznxE3FQv02a/hjp/3
a+i0GDT4ExSNuMxAqEewnWTymHS8bAsPGKuEMk9zElMZgeM6RrZUT+RL/ybjw5Mi
H8mP/tEcR1jAsm30BSoWV0nKHMXLpuOVTQS2V3ngzMWoA/l/9t7CafhkpV7IGfnB
HwQChc3L9fyZ/LwCo0WR1mHbzoPq+K4fwOnjdFEbgUSvfQ3+QiXXrfWt7C9IYAmA
/6cxo9vG1NH6sQ3BJiEyJNaWj3q2c5U=
=E+yp
-----END PGP PUBLIC KEY BLOCK-----
EOF

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

gpg_keys_official="file:///etc/pki/rpm-gpg/RPM-GPG-KEY-CentOS-7-aarch64 file:///etc/pki/rpm-gpg/RPM-GPG-KEY-CentOS-SIG-AltArch-Arm32 file:///etc/pki/rpm-gpg/RPM-GPG-KEY-CentOS-SIG-AltArch-7-ppc64"

	if [ -n "${GPG_KEYS}" ]; then
		echo gpgcheck=1 >> /etc/yum.repos.d/cdrom.repo
		echo gpgkey=${gpg_keys_official} ${GPG_KEYS} >> /etc/yum.repos.d/cdrom.repo
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
		return fmt.Errorf("Failed to run script: %w", err)
	}

	return nil
}

func (s *centOS) getRelease(URL, release, variant, arch string) (string, error) {
	releaseFields := strings.Split(release, ".")
	u := URL + path.Join("/", strings.ToLower(release), "isos", arch)

	var (
		resp *http.Response
		err  error
	)

	err = shared.Retry(func() error {
		resp, err = http.Get(u)
		if err != nil {
			return fmt.Errorf("Failed to get URL %q: %w", u, err)
		}

		return nil
	}, 3)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to read body: %w", err)
	}

	if len(releaseFields) == 3 && !strings.Contains(URL, "vault.centos.org") {
		return "", errors.New("Patch releases are only supported when using vault.centos.org as the mirror")
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
			return matches[len(matches)-1], nil
		}
	}

	return "", errors.New("Failed to find release")
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
