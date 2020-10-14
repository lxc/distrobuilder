package sources

import "github.com/lxc/distrobuilder/shared"

// A Downloader represents a source downloader.
type Downloader interface {
	Run(shared.Definition, string) error
}

// Get returns a Downloader.
func Get(name string) Downloader {
	switch name {
	case "alpinelinux-http":
		return NewAlpineLinuxHTTP()
	case "alt-http":
		return NewALTHTTP()
	case "apertis-http":
		return NewApertisHTTP()
	case "archlinux-http":
		return NewArchLinuxHTTP()
	case "centos-http":
		return NewCentOSHTTP()
	case "debootstrap":
		return NewDebootstrap()
	case "fedora-http":
		return NewFedoraHTTP()
	case "gentoo-http":
		return NewGentooHTTP()
	case "ubuntu-http":
		return NewUbuntuHTTP()
	case "sabayon-http":
		return NewSabayonHTTP()
	case "docker-http":
		return NewDockerHTTP()
	case "oraclelinux-http":
		return NewOracleLinuxHTTP()
	case "opensuse-http":
		return NewOpenSUSEHTTP()
	case "openwrt-http":
		return NewOpenWrtHTTP()
	case "plamolinux-http":
		return NewPlamoLinuxHTTP()
	case "voidlinux-http":
		return NewVoidLinuxHTTP()
	case "funtoo-http":
		return NewFuntooHTTP()
	case "windows":
		return NewWindows()
	}

	return nil
}
