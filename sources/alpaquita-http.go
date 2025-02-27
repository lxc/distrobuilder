package sources

import (
	"crypto/sha512"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/shared"
)

type alpaquita struct {
	common
}

func (s *alpaquita) Run() error {
	baseURL, fname, err := s.getMiniroot(s.definition)
	if err != nil {
		return err
	}

	var fpath string
	tarballURL := baseURL + fname
	if s.definition.Source.SkipVerification {
		fpath, err = s.DownloadHash(s.definition.Image,
			tarballURL, "", nil)
	} else {
		fpath, err = s.DownloadHash(s.definition.Image,
			tarballURL, tarballURL+".sha512", sha512.New())
	}
	if err != nil {
		return err
	}

	tarballLocal := filepath.Join(fpath, fname)
	s.logger.WithField("file", tarballLocal).Info("Unpacking image")

	err = shared.Unpack(tarballLocal, s.rootfsDir)
	if err != nil {
		return err
	}

	return nil
}

// Sample URLs (or with "latest" instead of date):
//
//	https://packages.bell-sw.com/alpaquita/musl/stream/releases/x86_64/alpaquita-minirootfs-stream-241231-musl-x86_64.tar.gz
//	https://packages.bell-sw.com/alpaquita/glibc/23/releases/aarch64/alpaquita-minirootfs-23-241231-glibc-aarch64.tar.gz
func (s *alpaquita) getMiniroot(definition shared.Definition) (string, string, error) {
	// default server
	if s.definition.Source.URL == "" {
		s.definition.Source.URL = "https://packages.bell-sw.com"
	}

	// normalize release name
	switch strings.ToLower(s.definition.Image.Release) {
	case "stream":
		s.definition.Image.Release = "stream"
	case "23", "23-lts":
		s.definition.Image.Release = "23"
	default:
		return "", "", fmt.Errorf("Unknown Alpaquita release %q", s.definition.Image.Release)
	}

	// make variant explicit and normalize it
	switch strings.ToLower(s.definition.Image.Variant) {
	case "musl", "default":
		s.definition.Image.Variant = "musl"
	case "glibc":
		s.definition.Image.Variant = "glibc"
	default:
		return "", "", fmt.Errorf("Unknown Alpaquita variant %q", s.definition.Image.Variant)
	}

	base := fmt.Sprintf("%s/alpaquita/%s/%s/releases/%s/",
		s.definition.Source.URL,
		s.definition.Image.Variant,
		s.definition.Image.Release,
		s.definition.Image.ArchitectureMapped)

	fname := fmt.Sprintf("alpaquita-minirootfs-%s-latest-%s-%s.tar.gz",
		s.definition.Image.Release,
		s.definition.Image.Variant,
		s.definition.Image.ArchitectureMapped)

	return base, fname, nil
}
