package sources

import "github.com/lxc/distrobuilder/shared"

// A Downloader represents a source downloader.
type Downloader interface {
	Run(shared.Definition, string, string, string) error
}

// Get returns a Downloader.
func Get(name string) Downloader {
	switch name {
	case "ubuntu-http":
		return NewUbuntuHTTP()
	case "debootstrap":
		return NewDebootstrap()
	case "archlinux-http":
		return NewArchLinuxHTTP()
	case "centos-http":
		return NewCentOSHTTP()
	case "alpinelinux-http":
		return NewAlpineLinuxHTTP()
	}

	return nil
}
