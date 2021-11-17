package sources

import (
	"crypto/sha256"
	"errors"
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
		return fmt.Errorf("Failed to download image: %w", err)
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
		err := shared.RunCommand(s.ctx, nil, nil, "unxz", "-k", filepath.Join(s.fpath, s.fname))
		if err != nil {
			return fmt.Errorf(`Failed to run "unxz": %w`, err)
		}
	}

	s.fname = strings.TrimSuffix(s.fname, ".xz")
	f := filepath.Join(s.fpath, s.fname)

	var out strings.Builder

	err := shared.RunCommand(s.ctx, nil, &out, "fdisk", "-l", "-o", "Start", f)
	if err != nil {
		return fmt.Errorf(`Failed to run "fdisk": %w`, err)
	}

	lines := strings.Split(out.String(), "\n")

	offset, err := strconv.Atoi(lines[len(lines)-2])
	if err != nil {
		return fmt.Errorf("Failed to convert %q: %w", lines[len(lines)-2], err)
	}

	imageDir := filepath.Join(s.cacheDir, "image")
	snapsDir := filepath.Join(s.cacheDir, "snaps")
	baseImageDir := fmt.Sprintf("%s.base", rootfsDir)

	for _, d := range []string{imageDir, snapsDir, baseImageDir} {
		err = os.MkdirAll(d, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", d, err)
		}
	}

	err = shared.RunCommand(s.ctx, nil, nil, "mount", "-o", fmt.Sprintf("loop,offset=%d", offset*512), f, imageDir)
	if err != nil {
		return fmt.Errorf("Failed to mount %q: %w", fmt.Sprintf("loop,offset=%d", offset*512), err)
	}
	defer unix.Unmount(imageDir, 0)

	err = shared.RsyncLocal(s.ctx, filepath.Join(imageDir, "system-data"), rootfsDir)
	if err != nil {
		return fmt.Errorf(`Failed to run "rsync": %w`, err)
	}

	// Create all the needed paths and links

	dirs := []string{"bin", "dev", "initrd", "lib", "mnt", "proc", "root", "sbin", "sys"}

	for _, d := range dirs {
		err := os.Mkdir(filepath.Join(rootfsDir, d), 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", filepath.Join(rootfsDir, d), err)
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
			return fmt.Errorf("Failed to create symlink %q: %w", l.link, err)
		}
	}

	baseDistro := "bionic"

	// Download the base Ubuntu image
	coreImage, err := getLatestCoreBaseImage("https://images.linuxcontainers.org/images", baseDistro, s.definition.Image.ArchitectureMapped)
	if err != nil {
		return fmt.Errorf("Failed to get latest core base image: %w", err)
	}

	_, err = s.DownloadHash(s.definition.Image, coreImage, "", sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", coreImage, err)
	}

	err = s.unpack(filepath.Join(s.fpath, "rootfs.tar.xz"), baseImageDir)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", filepath.Join(s.fpath, "rootfs.tar.xz"), err)
	}

	exitChroot, err := shared.SetupChroot(baseImageDir, shared.DefinitionEnv{}, nil)
	if err != nil {
		return fmt.Errorf("Failed to create chroot: %w", err)
	}

	err = shared.RunScript(s.ctx, `#!/bin/sh
	apt-get update
	apt-get install -y busybox-static fuse util-linux squashfuse
	`)
	if err != nil {
		exitChroot()
		return fmt.Errorf("Failed to run script: %w", err)
	}

	err = exitChroot()
	if err != nil {
		return fmt.Errorf("Failed to exit chroot: %w", err)
	}

	box := packr.New("ubuntu-core", "./data/ubuntu-core")

	file, err := box.Resolve("init")
	if err != nil {
		return fmt.Errorf(`Failed to resolve "init": %w`, err)
	}
	defer file.Close()

	target, err := os.Create(filepath.Join(rootfsDir, "bin", "init"))
	if err != nil {
		return fmt.Errorf("Failed to create %q: %w", filepath.Join(rootfsDir, "bin", "init"), err)
	}
	defer target.Close()

	_, err = io.Copy(target, file)
	if err != nil {
		return fmt.Errorf("Failed to copy %q to %q: %w", file.Name(), target.Name(), err)
	}

	err = target.Chmod(0755)
	if err != nil {
		return fmt.Errorf("Failed to chmod %q: %w", target.Name(), err)
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
			return fmt.Errorf("Failed to copy %q to %q: %w", b.source, b.target, err)
		}

		err = os.Chmod(b.target, 0755)
		if err != nil {
			return fmt.Errorf("Failed to chmod %q: %w", b.target, err)
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
			return fmt.Errorf("Failed to match pattern: %w", err)
		}

		if len(matches) != 1 {
			continue
		}

		dest := filepath.Join(rootfsDir, "lib", filepath.Base(matches[0]))

		source, err := os.Readlink(matches[0])
		if err != nil {
			return fmt.Errorf("Failed to read link %q: %w", matches[0], err)
		}

		// Build absolute path
		if !strings.HasPrefix(source, "/") {
			source = filepath.Join(filepath.Dir(matches[0]), source)
		}

		err = lxd.FileCopy(source, dest)
		if err != nil {
			return fmt.Errorf("Failed to copy %q to %q: %w", source, dest, err)
		}

		err = os.Chmod(dest, 0755)
		if err != nil {
			return fmt.Errorf("Failed to chmod %q: %w", dest, err)
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
				return fmt.Errorf("Failed to get latest release: %w", err)
			}
		}
	case "core":
		baseURL = fmt.Sprintf("%s/%s/stable/current/", s.definition.Source.URL, s.definition.Image.Release)
		s.fname = fmt.Sprintf("ubuntu-core-%s-%s.img.xz", s.definition.Image.Release, s.definition.Image.ArchitectureMapped)
	default:
		return fmt.Errorf("Unknown Ubuntu variant %q", s.definition.Image.Variant)
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("Failed to parse URL %q: %w", baseURL, err)
	}

	var fpath string

	checksumFile := ""
	// Force gpg checks when using http
	if !s.definition.Source.SkipVerification && url.Scheme != "https" {
		if len(s.definition.Source.Keys) == 0 {
			return errors.New("GPG keys are required if downloading from HTTP")
		}

		checksumFile = baseURL + "SHA256SUMS"
		fpath, err = s.DownloadHash(s.definition.Image, baseURL+"SHA256SUMS.gpg", "", nil)
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", baseURL+"SHA256SUMS.gpg", err)
		}

		_, err = s.DownloadHash(s.definition.Image, checksumFile, "", nil)
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", checksumFile, err)
		}

		valid, err := s.VerifyFile(
			filepath.Join(fpath, "SHA256SUMS"),
			filepath.Join(fpath, "SHA256SUMS.gpg"))
		if err != nil {
			return fmt.Errorf(`Failed to verify "SHA256SUMS": %w`, err)
		}
		if !valid {
			return errors.New(`Invalid signature for "SHA256SUMS"`)
		}
	}

	s.fpath, err = s.DownloadHash(s.definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", baseURL+s.fname, err)
	}

	return nil
}

func (s ubuntu) unpack(filePath, rootDir string) error {
	err := os.RemoveAll(rootDir)
	if err != nil {
		return fmt.Errorf("Failed to remove directory %q: %w", rootDir, err)
	}

	err = os.MkdirAll(rootDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", rootDir, err)
	}

	s.logger.Infow("Unpacking file", "file", filePath)

	err = lxd.Unpack(filePath, rootDir, false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", filePath, err)
	}

	return nil
}

func getLatestRelease(baseURL, release, arch string) (string, error) {
	var (
		resp *http.Response
		err  error
	)

	err = shared.Retry(func() error {
		resp, err = http.Get(baseURL)
		if err != nil {
			return fmt.Errorf("Failed to GET %q: %w", baseURL, err)
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
		return "", fmt.Errorf("Failed to parse URL %q: %w", fmt.Sprintf("%s/ubuntu/%s/%s/default", baseURL, release, arch), err)
	}

	var resp *http.Response

	err = shared.Retry(func() error {
		resp, err = http.Get(u.String())
		if err != nil {
			return fmt.Errorf("Failed to GET %q: %w", u.String(), err)
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

	regex := regexp.MustCompile(`\d{8}_\d{2}:\d{2}`)
	releases := regex.FindAllString(string(body), -1)

	if len(releases) > 1 {
		return fmt.Sprintf("%s/%s/rootfs.tar.xz", u.String(), releases[len(releases)-1]), nil
	}

	return "", errors.New("Failed to find latest core base image")
}
