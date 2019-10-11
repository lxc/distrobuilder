package sources

import (
	"bytes"
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
	"strconv"
	"strings"

	lxd "github.com/lxc/lxd/shared"
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
	s.majorVersion = strings.Split(definition.Image.Release, ".")[0]

	baseURL := fmt.Sprintf("%s/%s/isos/%s/", definition.Source.URL,
		s.majorVersion,
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
					checksumFile = "CHECKSUM.asc"
				} else {
					checksumFile = "sha256sum.txt.asc"
				}
			}

			fpath, err := shared.DownloadHash(definition.Image, baseURL+checksumFile, "", nil)
			if err != nil {
				return err
			}

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

	_, err = shared.DownloadHash(definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Error downloading CentOS image: %s", err)
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
	exitChroot, err := shared.SetupChroot(tempRootDir, shared.DefinitionEnv{})
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %s", err)
	}

	err = shared.RunScript(fmt.Sprintf(`
#!/bin/sh
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

	// Create cdrom repo for yum
	err = os.MkdirAll(filepath.Join(tempRootDir, "mnt", "cdrom"), 0755)
	if err != nil {
		return err
	}

	packagesDir := filepath.Join(isoDir, "Packages")
	repodataDir := filepath.Join(isoDir, "repodata")

	if !lxd.PathExists(packagesDir) {
		packagesDir = filepath.Join(isoDir, "BaseOS", "Packages")
	}
	if !lxd.PathExists(repodataDir) {
		repodataDir = filepath.Join(isoDir, "BaseOS", "repodata")
	}

	if !lxd.PathExists(packagesDir) {
		return fmt.Errorf("Missing Packages directory")
	}

	if !lxd.PathExists(repodataDir) {
		return fmt.Errorf("Missing repodata directory")
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
	gpgKeysPath := ""
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

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(tempRootDir, shared.DefinitionEnv{})
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %s", err)
	}

	err = shared.RunScript(fmt.Sprintf(`
#!/bin/sh
set -eux

GPG_KEYS="%s"

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

if [ -n "${GPG_KEYS}" ]; then
	echo gpgcheck=1 >> /etc/yum.repos.d/cdrom.repo
	echo gpgkey=${GPG_KEYS} >> /etc/yum.repos.d/cdrom.repo
else
	echo gpgcheck=0 >> /etc/yum.repos.d/cdrom.repo
fi

yum --disablerepo=* --enablerepo=cdrom -y reinstall yum

# Create a minimal rootfs
mkdir /rootfs
yum --installroot=/rootfs --disablerepo=* --enablerepo=cdrom -y --releasever=%s install basesystem centos-release yum
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

	var re []string
	if len(releaseFields) > 1 {
		if arch == "armhfp" {
			re = append(re, fmt.Sprintf("CentOS-Userland-%s.%s-armv7hl-RootFS-(?i:%s)(-\\d+)?-sda.raw.xz",
				releaseFields[0], releaseFields[1], variant))
		} else {
			re = append(re, fmt.Sprintf("CentOS-%s.%s-%s-(?i:%s)(-\\d+)?.iso",
				releaseFields[0], releaseFields[1], arch, variant))
			re = append(re, fmt.Sprintf("CentOS-%s-%s-%s-(?i:%s).iso",
				releaseFields[0], arch, releaseFields[1], variant))
		}
	} else {
		if arch == "armhfp" {
			re = append(re, fmt.Sprintf("CentOS-Userland-%s-armv7hl-RootFS-(?i:%s)(-\\d+)?-sda.raw.xz",
				releaseFields[0], variant))
		} else {
			re = append(re, fmt.Sprintf("CentOS-%s(.\\d+)?-%s-(?i:%s)(-\\d+)?.iso",
				releaseFields[0], arch, variant))
			re = append(re, fmt.Sprintf("CentOS-%s(.\\d+)?-%s(-\\d+)?-(?i:%s).iso",
				releaseFields[0], arch, variant))
		}
	}

	for _, r := range re {
		match := regexp.MustCompile(r).FindString(string(body))
		if match != "" {
			return match
		}
	}

	return ""
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
