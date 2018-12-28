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
	}

	return nil
}
