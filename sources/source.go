package sources

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"

	"github.com/lxc/distrobuilder/shared"
)

// ErrUnknownDownloader represents the unknown downloader error.
var ErrUnknownDownloader = errors.New("Unknown downloader")

type downloader interface {
	init(ctx context.Context, logger *logrus.Logger, definition shared.Definition, rootfsDir string, cacheDir string, sourcesDir string)

	Downloader
}

// Downloader represents a source downloader.
type Downloader interface {
	Run() error
}

var downloaders = map[string]func() downloader{
	"almalinux-http":       func() downloader { return &almalinux{} },
	"alpaquita-http":       func() downloader { return &alpaquita{} },
	"alpinelinux-http":     func() downloader { return &alpineLinux{} },
	"alt-http":             func() downloader { return &altLinux{} },
	"apertis-http":         func() downloader { return &apertis{} },
	"archlinux-http":       func() downloader { return &archlinux{} },
	"busybox":              func() downloader { return &busybox{} },
	"centos-http":          func() downloader { return &centOS{} },
	"debootstrap":          func() downloader { return &debootstrap{} },
	"docker-http":          func() downloader { return &docker{} },
	"fedora-http":          func() downloader { return &fedora{} },
	"funtoo-http":          func() downloader { return &funtoo{} },
	"gentoo-http":          func() downloader { return &gentoo{} },
	"nixos-http":           func() downloader { return &nixos{} },
	"openeuler-http":       func() downloader { return &openEuler{} },
	"opensuse-http":        func() downloader { return &opensuse{} },
	"openwrt-http":         func() downloader { return &openwrt{} },
	"oraclelinux-http":     func() downloader { return &oraclelinux{} },
	"plamolinux-http":      func() downloader { return &plamolinux{} },
	"rockylinux-http":      func() downloader { return &rockylinux{} },
	"rootfs-http":          func() downloader { return &rootfs{} },
	"rpmbootstrap":         func() downloader { return &rpmbootstrap{} },
	"springdalelinux-http": func() downloader { return &springdalelinux{} },
	"ubuntu-http":          func() downloader { return &ubuntu{} },
	"voidlinux-http":       func() downloader { return &voidlinux{} },
	"vyos-http":            func() downloader { return &vyos{} },
	"slackware-http":       func() downloader { return &slackware{} },
}

// Load loads and initializes a downloader.
func Load(ctx context.Context, downloaderName string, logger *logrus.Logger, definition shared.Definition, rootfsDir string, cacheDir string, sourcesDir string) (Downloader, error) {
	df, ok := downloaders[downloaderName]
	if !ok {
		return nil, ErrUnknownDownloader
	}

	d := df()

	d.init(ctx, logger, definition, rootfsDir, cacheDir, sourcesDir)

	return d, nil
}
