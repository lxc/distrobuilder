package generators

import (
	"errors"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

// ErrNotSupported returns a "Not supported" error
var ErrNotSupported = errors.New("Not supported")

// Generator interface.
type Generator interface {
	RunLXC(string, string, *image.LXCImage, shared.DefinitionTargetLXC, shared.DefinitionFile) error
	RunLXD(string, string, *image.LXDImage, shared.DefinitionTargetLXD, shared.DefinitionFile) error
	Run(string, string, shared.DefinitionFile) error
}

// Get returns a Generator.
func Get(generator string) Generator {
	switch generator {
	case "hostname":
		return HostnameGenerator{}
	case "hosts":
		return HostsGenerator{}
	case "remove":
		return RemoveGenerator{}
	case "dump":
		return DumpGenerator{}
	case "copy":
		return CopyGenerator{}
	case "template":
		return TemplateGenerator{}
	case "upstart-tty":
		return UpstartTTYGenerator{}
	case "cloud-init":
		return CloudInitGenerator{}
	case "lxd-agent":
		return LXDAgentGenerator{}
	case "fstab":
		return FstabGenerator{}
	}

	return nil
}
