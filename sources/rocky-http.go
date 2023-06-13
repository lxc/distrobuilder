package sources

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lxc/distrobuilder/shared"
)

type rockylinux struct {
	commonRHEL

	fname string
}

// Run downloads the tarball and unpacks it.
func (s *rockylinux) Run() error {
	var err error

	baseURL := fmt.Sprintf("%s/%s/isos/%s/", s.definition.Source.URL,
		strings.ToLower(s.definition.Image.Release),
		s.definition.Image.ArchitectureMapped)

	s.fname, err = s.getRelease(s.definition.Source.URL, s.definition.Image.Release,
		s.definition.Source.Variant, s.definition.Image.ArchitectureMapped)
	if err != nil {
		return fmt.Errorf("Failed to get release: %w", err)
	}

	fpath := s.getTargetDir()

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

			checksumFile = "CHECKSUM"

			_, err := s.DownloadHash(s.definition.Image, baseURL+checksumFile, "", nil)
			if err != nil {
				return fmt.Errorf("Failed to download %q: %w", baseURL+checksumFile, err)
			}
		}
	}

	_, err = s.DownloadHash(s.definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", baseURL+s.fname, err)
	}

	s.logger.WithField("file", filepath.Join(fpath, s.fname)).Info("Unpacking ISO")

	err = s.unpackISO(filepath.Join(fpath, s.fname), s.rootfsDir, s.isoRunner)
	if err != nil {
		return fmt.Errorf("Failed to unpack ISO: %w", err)
	}

	return nil
}

func (s *rockylinux) isoRunner(gpgKeysPath string) error {
	repoURL := "mirrorlist=http://mirrors.rockylinux.org/mirrorlist?arch=\\$basearch&repo=BaseOS-\\$releasever"

	if strings.Contains(s.definition.Source.URL, "/vault/") {
		repoURL = fmt.Sprintf("baseurl=http://dl.rockylinux.org/vault/rocky/%s/BaseOS/\\$basearch/os/", s.definition.Image.Release)
	}

	err := shared.RunScript(s.ctx, fmt.Sprintf(`#!/bin/sh
set -eux
GPG_KEYS="%s"
RELEASE="%s"
REPO_URL="%s"
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
	if [ "${RELEASE}" -eq 9 ]; then
		cat <<- "EOF" > /etc/pki/rpm-gpg/RPM-GPG-KEY-Rocky-9
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: resf.keykeeper.v1
Comment: Keykeeper

xsFNBGJ5RksBEADF/Lzssm7uryV6+VHAgL36klyCVcHwvx9Bk853LBOuHVEZWsme
kbJF3fQG7i7gfCKGuV5XW15xINToe4fBThZteGJziboSZRpkEQ2z3lYcbg34X7+d
co833lkBNgz1v6QO7PmAdY/x76Q6Hx0J9yiJWd+4j+vRi4hbWuh64vUtTd7rPwk8
0y3g4oK1YT0NR0Xm/QUO9vWmkSTVflQ6y82HhHIUrG+1vQnSOrWaC0O1lqUI3Nuo
b6jTARCmbaPsi+XVQnBbsnPPq6Tblwc+NYJSqj5d9nT0uEXT7Zovj4Je5oWVFXp9
P1OWkbo2z5XkKjoeobM/zKDESJR78h+YQAN9IOKFjL/u/Gzrk1oEgByCABXOX+H5
hfucrq5U3bbcKy4e5tYgnnZxqpELv3fN/2l8iZknHEh5aYNT5WXVHpD/8u2rMmwm
I9YTEMueEtmVy0ZV3opUzOlC+3ZUwjmvAJtdfJyeVW/VMy3Hw3Ih0Fij91rO613V
7n72ggVlJiX25jYyT4AXlaGfAOMndJNVgBps0RArOBYsJRPnvfHlLi5cfjVd7vYx
QhGX9ODYuvyJ/rW70dMVikeSjlBDKS08tvdqOgtiYy4yhtY4ijQC9BmCE9H9gOxU
FN297iLimAxr0EVsED96fP96TbDGILWsfJuxAvoqmpkElv8J+P1/F7to2QARAQAB
zU9Sb2NreSBFbnRlcnByaXNlIFNvZnR3YXJlIEZvdW5kYXRpb24gLSBSZWxlYXNl
IGtleSAyMDIyIDxyZWxlbmdAcm9ja3lsaW51eC5vcmc+wsGKBBMBCAA0BQJieUZL
FiEEIcslauFvxUxuZSlJcC1CbTUNJ10CGwMCHgECGQEDCwkHAhUIAxYAAgIiAQAK
CRBwLUJtNQ0nXWQ5D/9472seOyRO6//bQ2ns3w9lE+aTLlJ5CY0GSTb4xNuyv+AD
IXpgvLSMtTR0fp9GV3vMw6QIWsehDqt7O5xKWi+3tYdaXRpb1cvnh8r/oCcvI4uL
k8kImNgsx+Cj+drKeQo03vFxBTDi1BTQFkfEt32fA2Aw5gYcGElM717sNMAMQFEH
P+OW5hYDH4kcLbtUypPXFbcXUbaf6jUjfiEp5lLjqquzAyDPLlkzMr5RVa9n3/rI
R6OQp5loPVzCRZMgDLALBU2TcFXLVP+6hAW8qM77c+q/rOysP+Yd+N7GAd0fvEvA
mfeA4Y6dP0mMRu96EEAJ1qSKFWUul6K6nuqy+JTxktpw8F/IBAz44na17Tf02MJH
GCUWyM0n5vuO5kK+Ykkkwd+v43ZlqDnwG7akDkLwgj6O0QNx2TGkdgt3+C6aHN5S
MiF0pi0qYbiN9LO0e05Ai2r3zTFC/pCaBWlG1ph2jx1pDy4yUVPfswWFNfe5I+4i
CMHPRFsZNYxQnIA2Prtgt2YMwz3VIGI6DT/Z56Joqw4eOfaJTTQSXCANts/gD7qW
D3SZXPc7wQD63TpDEjJdqhmepaTECbxN7x/p+GwIZYWJN+AYhvrfGXfjud3eDu8/
i+YIbPKH1TAOMwiyxC106mIL705p+ORf5zATZMyB8Y0OvRIz5aKkBDFZM2QN6A==
=PzIf
-----END PGP PUBLIC KEY BLOCK-----
EOF
		# Override the GPG key as the one inside the ISO doesn't work.
		GPG_KEYS=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-Rocky-9
	fi

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
		if [ "${RELEASE}" -eq 8 ]; then
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
		else
			cat <<- "EOF" > /etc/pki/rpm-gpg/RPM-GPG-KEY-rockyofficial
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: resf.keykeeper.v1
Comment: Keykeeper

xsFNBGJ5RksBEADF/Lzssm7uryV6+VHAgL36klyCVcHwvx9Bk853LBOuHVEZWsme
kbJF3fQG7i7gfCKGuV5XW15xINToe4fBThZteGJziboSZRpkEQ2z3lYcbg34X7+d
co833lkBNgz1v6QO7PmAdY/x76Q6Hx0J9yiJWd+4j+vRi4hbWuh64vUtTd7rPwk8
0y3g4oK1YT0NR0Xm/QUO9vWmkSTVflQ6y82HhHIUrG+1vQnSOrWaC0O1lqUI3Nuo
b6jTARCmbaPsi+XVQnBbsnPPq6Tblwc+NYJSqj5d9nT0uEXT7Zovj4Je5oWVFXp9
P1OWkbo2z5XkKjoeobM/zKDESJR78h+YQAN9IOKFjL/u/Gzrk1oEgByCABXOX+H5
hfucrq5U3bbcKy4e5tYgnnZxqpELv3fN/2l8iZknHEh5aYNT5WXVHpD/8u2rMmwm
I9YTEMueEtmVy0ZV3opUzOlC+3ZUwjmvAJtdfJyeVW/VMy3Hw3Ih0Fij91rO613V
7n72ggVlJiX25jYyT4AXlaGfAOMndJNVgBps0RArOBYsJRPnvfHlLi5cfjVd7vYx
QhGX9ODYuvyJ/rW70dMVikeSjlBDKS08tvdqOgtiYy4yhtY4ijQC9BmCE9H9gOxU
FN297iLimAxr0EVsED96fP96TbDGILWsfJuxAvoqmpkElv8J+P1/F7to2QARAQAB
zU9Sb2NreSBFbnRlcnByaXNlIFNvZnR3YXJlIEZvdW5kYXRpb24gLSBSZWxlYXNl
IGtleSAyMDIyIDxyZWxlbmdAcm9ja3lsaW51eC5vcmc+wsGKBBMBCAA0BQJieUZL
FiEEIcslauFvxUxuZSlJcC1CbTUNJ10CGwMCHgECGQEDCwkHAhUIAxYAAgIiAQAK
CRBwLUJtNQ0nXWQ5D/9472seOyRO6//bQ2ns3w9lE+aTLlJ5CY0GSTb4xNuyv+AD
IXpgvLSMtTR0fp9GV3vMw6QIWsehDqt7O5xKWi+3tYdaXRpb1cvnh8r/oCcvI4uL
k8kImNgsx+Cj+drKeQo03vFxBTDi1BTQFkfEt32fA2Aw5gYcGElM717sNMAMQFEH
P+OW5hYDH4kcLbtUypPXFbcXUbaf6jUjfiEp5lLjqquzAyDPLlkzMr5RVa9n3/rI
R6OQp5loPVzCRZMgDLALBU2TcFXLVP+6hAW8qM77c+q/rOysP+Yd+N7GAd0fvEvA
mfeA4Y6dP0mMRu96EEAJ1qSKFWUul6K6nuqy+JTxktpw8F/IBAz44na17Tf02MJH
GCUWyM0n5vuO5kK+Ykkkwd+v43ZlqDnwG7akDkLwgj6O0QNx2TGkdgt3+C6aHN5S
MiF0pi0qYbiN9LO0e05Ai2r3zTFC/pCaBWlG1ph2jx1pDy4yUVPfswWFNfe5I+4i
CMHPRFsZNYxQnIA2Prtgt2YMwz3VIGI6DT/Z56Joqw4eOfaJTTQSXCANts/gD7qW
D3SZXPc7wQD63TpDEjJdqhmepaTECbxN7x/p+GwIZYWJN+AYhvrfGXfjud3eDu8/
i+YIbPKH1TAOMwiyxC106mIL705p+ORf5zATZMyB8Y0OvRIz5aKkBDFZM2QN6A==
=PzIf
-----END PGP PUBLIC KEY BLOCK-----
EOF
		fi
	fi
	cat <<- EOF > /etc/yum.repos.d/Rocky-BaseOS.repo
[BaseOS]
name=Rocky-\$releasever - Base
${REPO_URL}
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
`, gpgKeysPath, s.definition.Image.Release, repoURL))
	if err != nil {
		return fmt.Errorf("Failed to run ISO script: %w", err)
	}

	return nil
}

func (s *rockylinux) getRelease(URL, release, variant, arch string) (string, error) {
	u := URL + path.Join("/", strings.ToLower(release), "isos", arch)

	resp, err := http.Get(u)
	if err != nil {
		return "", fmt.Errorf("Failed to GET %q: %w", u, err)
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

	return "", errors.New("Failed to find release")
}

func (s *rockylinux) getRegexes(arch string, variant string, release string) []*regexp.Regexp {
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
