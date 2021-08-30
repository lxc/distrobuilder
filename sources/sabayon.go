package sources

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
)

type sabayon struct {
	common
}

// Run downloads a Sabayon tarball.
func (s *sabayon) Run() error {
	var baseURL string

	fname := fmt.Sprintf("Sabayon_Linux_DAILY_%s_tarball.tar.gz",
		s.definition.Image.ArchitectureMapped)
	tarballPath := fmt.Sprintf("%s/%s", s.definition.Source.URL, fname)

	resp, err := http.Head(tarballPath)
	if err != nil {
		return errors.WithMessagef(err, "Failed to HEAD %q", tarballPath)
	}

	baseURL, fname = path.Split(resp.Request.URL.String())

	url, err := url.Parse(fmt.Sprintf("%s/%s", baseURL, fname))
	if err != nil {
		return errors.WithMessagef(err, "Failed to parse URL %q", fmt.Sprintf("%s/%s", baseURL, fname))
	}

	var fpath string

	// From sabayon currently we have only MD5 checksum for now.
	if s.definition.Source.SkipVerification {
		fpath, err = shared.DownloadHash(s.definition.Image, url.String(), "", nil)
	} else {
		fpath, err = shared.DownloadHash(s.definition.Image, url.String(), url.String()+".md5", md5.New())
	}
	if err != nil {
		return errors.WithMessagef(err, "Failed to download %q", url.String())
	}

	s.logger.Infow("Unpacking image", "file", filepath.Join(fpath, fname))

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return errors.WithMessagef(err, "Failed to unpack %q", filepath.Join(fpath, fname))
	}

	return nil
}
