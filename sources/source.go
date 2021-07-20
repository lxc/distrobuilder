package sources

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/lxc/distrobuilder/shared"
)

// ErrUnknownDownloader represents the unknown downloader error
var ErrUnknownDownloader = errors.New("Unknown downloader")

type downloader interface {
	init(logger *zap.SugaredLogger, definition shared.Definition, rootfsDir string, cacheDir string)

	Downloader
}

// Downloader represents a source downloader.
type Downloader interface {
	Run() error
}

var downloaders = map[string]func() downloader{
	"almalinux-http":       func() downloader { return &almalinux{} },
	"alpinelinux-http":     func() downloader { return &alpineLinux{} },
	"alt-http":             func() downloader { return &altLinux{} },
	"apertis-http":         func() downloader { return &apertis{} },
	"archlinux-http":       func() downloader { return &archlinux{} },
	"centos-http":          func() downloader { return &centOS{} },
	"debootstrap":          func() downloader { return &debootstrap{} },
	"docker-http":          func() downloader { return &docker{} },
	"fedora-http":          func() downloader { return &fedora{} },
	"funtoo-http":          func() downloader { return &funtoo{} },
	"gentoo-http":          func() downloader { return &gentoo{} },
	"opensuse-http":        func() downloader { return &opensuse{} },
	"openwrt-http":         func() downloader { return &openwrt{} },
	"oraclelinux-http":     func() downloader { return &oraclelinux{} },
	"plamolinux-http":      func() downloader { return &plamolinux{} },
	"rockylinux-http":      func() downloader { return &rockylinux{} },
	"rootfs-http":          func() downloader { return &rootfs{} },
	"sabayon-http":         func() downloader { return &sabayon{} },
	"springdalelinux-http": func() downloader { return &springdalelinux{} },
	"ubuntu-http":          func() downloader { return &ubuntu{} },
	"voidlinux-http":       func() downloader { return &voidlinux{} },
}

// Load loads and initializes a downloader.
func Load(downloaderName string, logger *zap.SugaredLogger, definition shared.Definition, rootfsDir string, cacheDir string) (Downloader, error) {
	df, ok := downloaders[downloaderName]
	if !ok {
		return nil, ErrUnknownDownloader
	}

	d := df()

	d.init(logger, definition, rootfsDir, cacheDir)

	return d, nil
}
