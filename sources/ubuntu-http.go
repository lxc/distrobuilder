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

// UbuntuHTTP represents the Ubuntu HTTP downloader.
type UbuntuHTTP struct {
	fname string
	fpath string
}

// NewUbuntuHTTP creates a new UbuntuHTTP instance.
func NewUbuntuHTTP() *UbuntuHTTP {
	return &UbuntuHTTP{}
}

// Run downloads the tarball and unpacks it.
func (s *UbuntuHTTP) Run(definition shared.Definition, rootfsDir string) error {
	err := s.downloadImage(definition)
	if err != nil {
		return err
	}

	switch strings.ToLower(definition.Source.Variant) {
	case "core":
		return s.runCoreVariant(definition, rootfsDir)
	}

	return s.runDefaultVariant(definition, rootfsDir)
}

func (s *UbuntuHTTP) runDefaultVariant(definition shared.Definition, rootfsDir string) error {
	err := s.unpack(filepath.Join(s.fpath, s.fname), rootfsDir)
	if err != nil {
		return err
	}

	if definition.Source.AptSources != "" {
		// Run the template
		out, err := shared.RenderTemplate(definition.Source.AptSources, definition)
		if err != nil {
			return err
		}

		// Append final new line if missing
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}

		// Replace content of sources.list with the templated content.
		file, err := os.Create(filepath.Join(rootfsDir, "etc", "apt", "sources.list"))
		if err != nil {
			return err
		}
		defer file.Close()

		file.WriteString(out)
	}

	return nil
}

func (s *UbuntuHTTP) runCoreVariant(definition shared.Definition, rootfsDir string) error {
	if !lxd.PathExists(filepath.Join(s.fpath, strings.TrimSuffix(s.fname, ".xz"))) {
		err := shared.RunCommand("unxz", "-k", filepath.Join(s.fpath, s.fname))
		if err != nil {
			return err
		}
	}

	s.fname = strings.TrimSuffix(s.fname, ".xz")
	f := filepath.Join(s.fpath, s.fname)

	output, err := lxd.RunCommand("fdisk", "-l", "-o", "Start", f)
	if err != nil {
		return err
	}

	lines := strings.Split(output, "\n")

	offset, err := strconv.Atoi(lines[len(lines)-2])
	if err != nil {
		return err
	}

	imageDir := filepath.Join(os.TempDir(), "distrobuilder", "image")
	snapsDir := filepath.Join(os.TempDir(), "distrobuilder", "snaps")
	baseImageDir := fmt.Sprintf("%s.base", rootfsDir)

	os.MkdirAll(imageDir, 0755)
	os.MkdirAll(snapsDir, 0755)
	os.MkdirAll(baseImageDir, 0755)
	defer os.RemoveAll(filepath.Join(os.TempDir(), "distrobuilder"))
	defer os.RemoveAll(filepath.Join(baseImageDir, "rootfs"))

	err = shared.RunCommand("mount", "-o", fmt.Sprintf("loop,offset=%d", offset*512), f, imageDir)
	if err != nil {
		return err
	}
	defer unix.Unmount(imageDir, 0)

	err = shared.RunCommand("rsync", "-qa", filepath.Join(imageDir, "system-data"), rootfsDir)
	if err != nil {
		return err
	}

	// Create all the needed paths and links

	dirs := []string{"bin", "dev", "initrd", "lib", "mnt", "proc", "root", "sbin", "sys"}

	for _, d := range dirs {
		err := os.Mkdir(filepath.Join(rootfsDir, d), 0755)
		if err != nil {
			return err
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
			return err
		}
	}

	baseDistro := "xenial"

	if definition.Image.Release == "18" {
		baseDistro = "bionic"
	}

	// Download the base Ubuntu image
	coreImage := getLatestCoreBaseImage("https://images.linuxcontainers.org/images", baseDistro, definition.Image.ArchitectureMapped)

	_, err = shared.DownloadHash(definition.Image, coreImage, "", sha256.New())
	if err != nil {
		return fmt.Errorf("Error downloading base Ubuntu image: %s", err)
	}

	err = s.unpack(filepath.Join(s.fpath, "rootfs.tar.xz"), baseImageDir)
	if err != nil {
		return err
	}

	exitChroot, err := shared.SetupChroot(baseImageDir, shared.DefinitionEnv{})
	if err != nil {
		return err
	}

	err = shared.RunScript(`
	#!/bin/sh
	apt-get update
	apt-get install -y busybox-static fuse util-linux squashfuse
	`)
	if err != nil {
		exitChroot()
		return err
	}

	err = exitChroot()
	if err != nil {
		return err
	}

	box := packr.New("ubuntu-core", "./data/ubuntu-core")

	file, err := box.Resolve("init")
	if err != nil {
		return err
	}
	defer file.Close()

	target, err := os.Create(filepath.Join(rootfsDir, "bin", "init"))
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, file)
	if err != nil {
		return err
	}

	err = target.Chmod(0755)
	if err != nil {
		return err
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
			return err
		}

		err = os.Chmod(b.target, 0755)
		if err != nil {
			return err
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
			return err
		}

		if len(matches) != 1 {
			continue
		}

		dest := filepath.Join(rootfsDir, "lib", filepath.Base(matches[0]))

		source, err := os.Readlink(matches[0])
		if err != nil {
			return err
		}

		// Build absolute path
		if !strings.HasPrefix(source, "/") {
			source = filepath.Join(filepath.Dir(matches[0]), source)
		}

		err = lxd.FileCopy(source, dest)
		if err != nil {
			return err
		}

		err = os.Chmod(dest, 0755)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *UbuntuHTTP) downloadImage(definition shared.Definition) error {
	var baseURL string

	switch strings.ToLower(definition.Image.Variant) {
	case "default":
		baseURL = fmt.Sprintf("%s/releases/%s/release/", definition.Source.URL,
			definition.Image.Release)

		if strings.ContainsAny(definition.Image.Release, "0123456789") {
			s.fname = fmt.Sprintf("ubuntu-base-%s-base-%s.tar.gz",
				definition.Image.Release, definition.Image.ArchitectureMapped)
		} else {
			// if release is non-numerical, find the latest release
			s.fname = getLatestRelease(baseURL,
				definition.Image.Release, definition.Image.ArchitectureMapped)
			if s.fname == "" {
				return fmt.Errorf("Couldn't find latest release")
			}
		}
	case "core":
		baseURL = fmt.Sprintf("%s/%s/stable/current/", definition.Source.URL, definition.Image.Release)
		s.fname = fmt.Sprintf("ubuntu-core-%s-%s.img.xz", definition.Image.Release, definition.Image.ArchitectureMapped)
	default:
		return fmt.Errorf("Unknown Ubuntu variant: %s", definition.Image.Variant)
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	var fpath string

	checksumFile := ""
	// Force gpg checks when using http
	if !definition.Source.SkipVerification && url.Scheme != "https" {
		if len(definition.Source.Keys) == 0 {
			return errors.New("GPG keys are required if downloading from HTTP")
		}

		checksumFile = baseURL + "SHA256SUMS"
		fpath, err = shared.DownloadHash(definition.Image, baseURL+"SHA256SUMS.gpg", "", nil)
		if err != nil {
			return err
		}

		shared.DownloadHash(definition.Image, checksumFile, "", nil)

		valid, err := shared.VerifyFile(
			filepath.Join(fpath, "SHA256SUMS"),
			filepath.Join(fpath, "SHA256SUMS.gpg"),
			definition.Source.Keys,
			definition.Source.Keyserver)
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("Failed to validate tarball")
		}
	}

	s.fpath, err = shared.DownloadHash(definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Error downloading Ubuntu image: %s", err)
	}

	return nil
}

func (s UbuntuHTTP) unpack(filePath, rootDir string) error {
	os.RemoveAll(rootDir)
	os.MkdirAll(rootDir, 0755)

	err := lxd.Unpack(filePath, rootDir, false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to unpack tarball: %s", err)
	}

	return nil
}

func getLatestRelease(baseURL, release, arch string) string {
	resp, err := http.Get(baseURL)
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

	regex := regexp.MustCompile(fmt.Sprintf("ubuntu-base-\\d{2}\\.\\d{2}(\\.\\d+)?-base-%s.tar.gz", arch))
	releases := regex.FindAllString(string(body), -1)

	if len(releases) > 1 {
		return string(releases[len(releases)-1])
	}

	return ""
}

func getLatestCoreBaseImage(baseURL, release, arch string) string {
	u, err := url.Parse(fmt.Sprintf("%s/ubuntu/%s/%s/default", baseURL, release, arch))
	if err != nil {
		return ""
	}

	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	regex := regexp.MustCompile(`\d{8}_\d{2}:\d{2}`)
	releases := regex.FindAllString(string(body), -1)

	if len(releases) > 1 {
		return fmt.Sprintf("%s/%s/rootfs.tar.xz", u.String(), releases[len(releases)-1])
	}

	return ""
}
