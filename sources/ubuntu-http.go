package sources

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gobuffalo/packr/v2"
	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/shared"
)

type ubuntu struct {
	common

	fname string
	fpath string
}

// Run downloads the tarball and unpacks it.
func (s *ubuntu) Run() error {
	err := s.downloadImage(s.definition)
	if err != nil {
		return errors.Wrap(err, "Failed to download image")
	}

	switch strings.ToLower(s.definition.Source.Variant) {
	case "core":
		return s.runCoreVariant(s.definition, s.rootfsDir)
	}

	return s.runDefaultVariant(s.definition, s.rootfsDir)
}

func (s *ubuntu) runDefaultVariant(definition shared.Definition, rootfsDir string) error {
	return s.unpack(filepath.Join(s.fpath, s.fname), rootfsDir)
}

func (s *ubuntu) runCoreVariant(definition shared.Definition, rootfsDir string) error {
	if !lxd.PathExists(filepath.Join(s.fpath, strings.TrimSuffix(s.fname, ".xz"))) {
		err := shared.RunCommand("unxz", "-k", filepath.Join(s.fpath, s.fname))
		if err != nil {
			return errors.Wrapf(err, `Failed to run "unxz"`)
		}
	}

	s.fname = strings.TrimSuffix(s.fname, ".xz")
	f := filepath.Join(s.fpath, s.fname)

	output, err := lxd.RunCommand("fdisk", "-l", "-o", "Start", f)
	if err != nil {
		return errors.Wrap(err, `Failed to run "fdisk"`)
	}

	lines := strings.Split(output, "\n")

	offset, err := strconv.Atoi(lines[len(lines)-2])
	if err != nil {
		return errors.Wrapf(err, "Failed to convert %q", lines[len(lines)-2])
	}

	imageDir := filepath.Join(os.TempDir(), "distrobuilder", "image")
	snapsDir := filepath.Join(os.TempDir(), "distrobuilder", "snaps")
	baseImageDir := fmt.Sprintf("%s.base", rootfsDir)

	defer os.RemoveAll(filepath.Join(os.TempDir(), "distrobuilder"))
	defer os.RemoveAll(filepath.Join(baseImageDir, "rootfs"))

	for _, d := range []string{imageDir, snapsDir, baseImageDir} {
		err = os.MkdirAll(d, 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to create directory %q", d)
		}
	}

	err = shared.RunCommand("mount", "-o", fmt.Sprintf("loop,offset=%d", offset*512), f, imageDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to mount %q", fmt.Sprintf("loop,offset=%d", offset*512))
	}
	defer unix.Unmount(imageDir, 0)

	err = shared.RunCommand("rsync", "-qa", filepath.Join(imageDir, "system-data"), rootfsDir)
	if err != nil {
		return errors.Wrap(err, `Failed to run "rsync"`)
	}

	// Create all the needed paths and links

	dirs := []string{"bin", "dev", "initrd", "lib", "mnt", "proc", "root", "sbin", "sys"}

	for _, d := range dirs {
		err := os.Mkdir(filepath.Join(rootfsDir, d), 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to create directory %q", filepath.Join(rootfsDir, d))
		}
	}

	links := []struct {
		target string
		link   string
	}{
		{
			"lib",
			filepath.Join(rootfsDir, "lib64"),
		},
		{
			"/bin/busybox",
			filepath.Join(rootfsDir, "bin", "sh"),
		},
		{
			"/bin/init",
			filepath.Join(rootfsDir, "sbin", "init"),
		},
	}

	for _, l := range links {
		err = os.Symlink(l.target, l.link)
		if err != nil {
			return errors.Wrapf(err, "Failed to create symlink %q", l.link)
		}
	}

	baseDistro := "bionic"

	// Download the base Ubuntu image
	coreImage, err := getLatestCoreBaseImage("https://images.linuxcontainers.org/images", baseDistro, s.definition.Image.ArchitectureMapped)
	if err != nil {
		return errors.Wrap(err, "Failed to get latest core base image")
	}

	_, err = shared.DownloadHash(s.definition.Image, coreImage, "", sha256.New())
	if err != nil {
		return errors.Wrapf(err, "Failed to download %q", coreImage)
	}

	err = s.unpack(filepath.Join(s.fpath, "rootfs.tar.xz"), baseImageDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to unpack %q", filepath.Join(s.fpath, "rootfs.tar.xz"))
	}

	exitChroot, err := shared.SetupChroot(baseImageDir, shared.DefinitionEnv{}, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create chroot")
	}

	err = shared.RunScript(`#!/bin/sh
	apt-get update
	apt-get install -y busybox-static fuse util-linux squashfuse
	`)
	if err != nil {
		exitChroot()
		return errors.Wrap(err, "Failed to run script")
	}

	err = exitChroot()
	if err != nil {
		return errors.Wrap(err, "Failed to exit chroot")
	}

	box := packr.New("ubuntu-core", "./data/ubuntu-core")

	file, err := box.Resolve("init")
	if err != nil {
		return errors.Wrap(err, `Failed to resolve "init"`)
	}
	defer file.Close()

	target, err := os.Create(filepath.Join(rootfsDir, "bin", "init"))
	if err != nil {
		return errors.Wrapf(err, "Failed to create %q", filepath.Join(rootfsDir, "bin", "init"))
	}
	defer target.Close()

	_, err = io.Copy(target, file)
	if err != nil {
		return errors.Wrapf(err, "Failed to copy %q to %q", file.Name(), target.Name())
	}

	err = target.Chmod(0755)
	if err != nil {
		return errors.Wrapf(err, "Failed to chmod %q", target.Name())
	}

	// Copy system binaries

	binaries := []struct {
		source string
		target string
	}{
		{
			filepath.Join(baseImageDir, "bin", "busybox"),
			filepath.Join(rootfsDir, "bin", "busybox"),
		},
		{
			filepath.Join(baseImageDir, "bin", "cpio"),
			filepath.Join(rootfsDir, "bin", "cpio"),
		},
		{
			filepath.Join(baseImageDir, "sbin", "mount.fuse"),
			filepath.Join(rootfsDir, "bin", "mount.fuse"),
		},
		{
			filepath.Join(baseImageDir, "sbin", "pivot_root"),
			filepath.Join(rootfsDir, "bin", "pivot_root"),
		},
		{
			filepath.Join(baseImageDir, "usr", "bin", "squashfuse"),
			filepath.Join(rootfsDir, "bin", "squashfuse"),
		},
	}

	for _, b := range binaries {
		err := lxd.FileCopy(b.source, b.target)
		if err != nil {
			return errors.Wrapf(err, "Failed to copy %q to %q", b.source, b.target)
		}

		err = os.Chmod(b.target, 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to chmod %q", b.target)
		}
	}

	// Copy needed libraries

	patterns := []string{
		"/lib/*-linux-gnu/ld-linux*.so.2",
		"/lib/*-linux-gnu/libc.so.6",
		"/lib/*-linux-gnu/libdl.so.2",
		"/lib/*-linux-gnu/libfuse.so.2",
		"/usr/lib/*-linux-gnu/liblz4.so.1",
		"/lib/*-linux-gnu/liblzma.so.5",
		"/lib/*-linux-gnu/liblzo2.so.2",
		"/lib/*-linux-gnu/libpthread.so.0",
		"/lib/*-linux-gnu/libz.so.1",
	}

	for _, p := range patterns {
		matches, err := filepath.Glob(filepath.Join(baseImageDir, p))
		if err != nil {
			return errors.Wrap(err, "Failed to match pattern")
		}

		if len(matches) != 1 {
			continue
		}

		dest := filepath.Join(rootfsDir, "lib", filepath.Base(matches[0]))

		source, err := os.Readlink(matches[0])
		if err != nil {
			return errors.Wrapf(err, "Failed to read link %q", matches[0])
		}

		// Build absolute path
		if !strings.HasPrefix(source, "/") {
			source = filepath.Join(filepath.Dir(matches[0]), source)
		}

		err = lxd.FileCopy(source, dest)
		if err != nil {
			return errors.Wrapf(err, "Failed to copy %q to %q", source, dest)
		}

		err = os.Chmod(dest, 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to chmod %q", dest)
		}
	}

	return nil
}

func (s *ubuntu) downloadImage(definition shared.Definition) error {
	var baseURL string
	var err error

	switch strings.ToLower(s.definition.Image.Variant) {
	case "default":
		baseURL = fmt.Sprintf("%s/releases/%s/release/", s.definition.Source.URL,
			s.definition.Image.Release)

		if strings.ContainsAny(s.definition.Image.Release, "0123456789") {
			s.fname = fmt.Sprintf("ubuntu-base-%s-base-%s.tar.gz",
				s.definition.Image.Release, s.definition.Image.ArchitectureMapped)
		} else {
			// if release is non-numerical, find the latest release
			s.fname, err = getLatestRelease(baseURL,
				s.definition.Image.Release, s.definition.Image.ArchitectureMapped)
			if err != nil {
				return errors.Wrap(err, "Failed to get latest release")
			}
		}
	case "core":
		baseURL = fmt.Sprintf("%s/%s/stable/current/", s.definition.Source.URL, s.definition.Image.Release)
		s.fname = fmt.Sprintf("ubuntu-core-%s-%s.img.xz", s.definition.Image.Release, s.definition.Image.ArchitectureMapped)
	default:
		return errors.Errorf("Unknown Ubuntu variant %q", s.definition.Image.Variant)
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse URL %q", baseURL)
	}

	var fpath string

	checksumFile := ""
	// Force gpg checks when using http
	if !s.definition.Source.SkipVerification && url.Scheme != "https" {
		if len(s.definition.Source.Keys) == 0 {
			return errors.New("GPG keys are required if downloading from HTTP")
		}

		checksumFile = baseURL + "SHA256SUMS"
		fpath, err = shared.DownloadHash(s.definition.Image, baseURL+"SHA256SUMS.gpg", "", nil)
		if err != nil {
			return errors.Wrapf(err, "Failed to download %q", baseURL+"SHA256SUMS.gpg")
		}

		_, err = shared.DownloadHash(s.definition.Image, checksumFile, "", nil)
		if err != nil {
			return errors.Wrapf(err, "Failed to download %q", checksumFile)
		}

		valid, err := shared.VerifyFile(
			filepath.Join(fpath, "SHA256SUMS"),
			filepath.Join(fpath, "SHA256SUMS.gpg"),
			s.definition.Source.Keys,
			s.definition.Source.Keyserver)
		if err != nil {
			return errors.Wrap(err, `Failed to verify "SHA256SUMS"`)
		}
		if !valid {
			return errors.New(`Invalid signature for "SHA256SUMS"`)
		}
	}

	s.fpath, err = shared.DownloadHash(s.definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return errors.Wrapf(err, "Failed to download %q", baseURL+s.fname)
	}

	return nil
}

func (s ubuntu) unpack(filePath, rootDir string) error {
	err := os.RemoveAll(rootDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to remove directory %q", rootDir)
	}

	err = os.MkdirAll(rootDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "Failed to create directory %q", rootDir)
	}

	err = lxd.Unpack(filePath, rootDir, false, false, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to unpack %q", filePath)
	}

	return nil
}

func getLatestRelease(baseURL, release, arch string) (string, error) {
	resp, err := http.Get(baseURL)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to GET %q", baseURL)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read body")
	}

	regex := regexp.MustCompile(fmt.Sprintf("ubuntu-base-\\d{2}\\.\\d{2}(\\.\\d+)?-base-%s.tar.gz", arch))
	releases := regex.FindAllString(string(body), -1)

	if len(releases) > 1 {
		return string(releases[len(releases)-1]), nil
	}

	return "", errors.New("Failed to find latest release")
}

func getLatestCoreBaseImage(baseURL, release, arch string) (string, error) {
	u, err := url.Parse(fmt.Sprintf("%s/ubuntu/%s/%s/default", baseURL, release, arch))
	if err != nil {
		return "", errors.Wrapf(err, "Failed to parse URL %q", fmt.Sprintf("%s/ubuntu/%s/%s/default", baseURL, release, arch))
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return "", errors.Wrapf(err, "Failed to GET %q", u.String())
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read body")
	}

	regex := regexp.MustCompile(`\d{8}_\d{2}:\d{2}`)
	releases := regex.FindAllString(string(body), -1)

	if len(releases) > 1 {
		return fmt.Sprintf("%s/%s/rootfs.tar.xz", u.String(), releases[len(releases)-1]), nil
	}

	return "", errors.New("Failed to find latest core base image")
}
