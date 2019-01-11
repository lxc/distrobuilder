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
	"syscall"

	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
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
	s.majorVersion = strings.Split(definition.Image.Release, ".")[0]

	baseURL := fmt.Sprintf("%s/%s/isos/%s/", definition.Source.URL,
		s.majorVersion,
		definition.Image.ArchitectureMapped)

	s.fname = s.getRelease(definition.Source.URL, definition.Image.Release,
		definition.Source.Variant, definition.Image.ArchitectureMapped)
	if s.fname == "" {
		return fmt.Errorf("Couldn't get name of iso")
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

			checksumFile = "sha256sum.txt.asc"
			shared.DownloadHash(baseURL+checksumFile, "", nil)
			valid, err := shared.VerifyFile(filepath.Join(os.TempDir(), checksumFile), "",
				definition.Source.Keys, definition.Source.Keyserver)
			if err != nil {
				return err
			}
			if !valid {
				return errors.New("Failed to verify tarball")
			}
		}
	}

	err = shared.DownloadHash(baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Error downloading CentOS image: %s", err)
	}

	return s.unpack(filepath.Join(os.TempDir(), s.fname), rootfsDir)
}

func (s CentOSHTTP) unpack(filePath, rootfsDir string) error {
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
	if lxd.PathExists(squashfsImage) {
		// The squashfs.img contains an image containing the rootfs, so first
		// mount squashfs.img
		err = shared.RunCommand("mount", "-o", "ro", squashfsImage, squashfsDir)
		if err != nil {
			return err
		}
		defer syscall.Unmount(squashfsDir, 0)

		rootfsImage = filepath.Join(squashfsDir, "LiveOS", "rootfs.img")
	} else {
		rootfsImage = filepath.Join(isoDir, "images", "install.img")
	}

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

	// Create cdrom repo for yum
	err = os.MkdirAll(filepath.Join(tempRootDir, "mnt", "cdrom"), 0755)
	if err != nil {
		return err
	}

	// Copy repo relevant files to the cdrom
	err = shared.RunCommand("rsync", "-qa",
		filepath.Join(isoDir, "Packages"),
		filepath.Join(isoDir, "repodata"),
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
		err = shared.RunCommand("rsync", "-qa", key,
			filepath.Join(tempRootDir, "mnt", "cdrom"))
		if err != nil {
			return err
		}
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(tempRootDir, shared.DefinitionEnv{})
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %s", err)
	}

	err = shared.RunScript(fmt.Sprintf(`
#!/bin/sh

# Create required files
touch /etc/mtab /etc/fstab

# Install initial package set
cd /mnt/cdrom/Packages
rpm -ivh --nodeps $(ls rpm-*.rpm | head -n1)
rpm -ivh --nodeps $(ls yum-*.rpm | head -n1)

# Add cdrom repo
mkdir -p /etc/yum.repos.d
cat <<- EOF > /etc/yum.repos.d/cdrom.repo
[cdrom]
name=Install CD-ROM
baseurl=file:///mnt/cdrom
enabled=0
gpgcheck=1
EOF

echo gpgkey=file:///mnt/cdrom/RPM-GPG-KEY-CentOS-%s >> /etc/yum.repos.d/cdrom.repo

yum --disablerepo=* --enablerepo=cdrom -y reinstall yum

# Create a minimal rootfs
mkdir /rootfs
yum --installroot=/rootfs --disablerepo=* --enablerepo=cdrom -y --releasever=%s install basesystem yum
rm -rf /rootfs/var/cache/yum
`, s.majorVersion, s.majorVersion))
	if err != nil {
		exitChroot()
		return err
	}

	exitChroot()

	err = shared.RunCommand("rsync", "-qa", tempRootDir+"/rootfs/", rootfsDir)

	return nil
}

func (s CentOSHTTP) getRelease(URL, release, variant, arch string) string {
	releaseFields := strings.Split(release, ".")
	resp, err := http.Get(URL + path.Join("/", releaseFields[0], "isos", arch))
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

	var re string
	if len(releaseFields) > 1 {
		re = fmt.Sprintf("CentOS-%s.%s-%s-(?i:%s)(-\\d+)?.iso",
			releaseFields[0], releaseFields[1], arch, variant)
	} else {
		re = fmt.Sprintf("CentOS-%s(.\\d+)?-%s-(?i:%s)(-\\d+)?.iso",
			releaseFields[0], arch, variant)
	}

	return regexp.MustCompile(re).FindString(string(body))
}
