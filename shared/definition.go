package shared

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/lxc/lxd/shared"
)

// A DefinitionPackages list packages which are to be either installed or
// removed.
type DefinitionPackages struct {
	Manager string   `yaml:"manager"`
	Install []string `yaml:"install,omitempty"`
	Remove  []string `yaml:"remove,omitempty"`
	Update  bool     `yaml:"update,omitempty"`
}

// A DefinitionImage represents the image.
type DefinitionImage struct {
	Description  string `yaml:"description"`
	Distribution string `yaml:"distribution"`
	Release      string `yaml:"release,omitempty"`
	Arch         string `yaml:"arch,omitempty"`
	Expiry       string `yaml:"expiry,omitempty"`
	Variant      string `yaml:"variant,omitempty"`
	Name         string `yaml:"name,omitempty"`
}

// A DefinitionSource specifies the download type and location
type DefinitionSource struct {
	Downloader string   `yaml:"downloader"`
	URL        string   `yaml:"url,omitempty"`
	Keys       []string `yaml:"keys,omitempty"`
	Keyserver  string   `yaml:"keyserver,omitempty"`
	Variant    string   `yaml:"variant,omitempty"`
	Suite      string   `yaml:"suite,omitempty"`
	AptSources string   `yaml:"apt_sources,omitempty"`
}

// A DefinitionTargetLXCConfig represents the config part of the metadata.
type DefinitionTargetLXCConfig struct {
	Type    string `yaml:"type"`
	Before  uint   `yaml:"before,omitempty"`
	After   uint   `yaml:"after,omitempty"`
	Content string `yaml:"content"`
}

// A DefinitionTargetLXC represents LXC specific files as part of the metadata.
type DefinitionTargetLXC struct {
	CreateMessage string                      `yaml:"create-message,omitempty"`
	Config        []DefinitionTargetLXCConfig `yaml:"config,omitempty"`
}

// A DefinitionTarget specifies target dependent files.
type DefinitionTarget struct {
	LXC DefinitionTargetLXC `yaml:"lxc,omitempty"`
}

// A DefinitionFile represents a file which is to be created inside to chroot.
type DefinitionFile struct {
	Generator string   `yaml:"generator"`
	Path      string   `yaml:"path,omitempty"`
	Content   string   `yaml:"content,omitempty"`
	Releases  []string `yaml:"releases,omitempty"`
}

// A DefinitionAction specifies a custom action (script) which is to be run after
// a certain action.
type DefinitionAction struct {
	Trigger  string   `yaml:"trigger"`
	Action   string   `yaml:"action"`
	Releases []string `yaml:"releases,omitempty"`
}

// DefinitionMappings defines custom mappings.
type DefinitionMappings struct {
	Architectures   map[string]string `yaml:"architectures,omitempty"`
	ArchitectureMap string            `yaml:"architecture_map,omitempty"`
}

// A Definition a definition.
type Definition struct {
	Image    DefinitionImage    `yaml:"image"`
	Source   DefinitionSource   `yaml:"source"`
	Targets  DefinitionTarget   `yaml:"targets,omitempty"`
	Files    []DefinitionFile   `yaml:"files,omitempty"`
	Packages DefinitionPackages `yaml:"packages,omitempty"`
	Actions  []DefinitionAction `yaml:"actions,omitempty"`
	Mappings DefinitionMappings `yaml:"mappings,omitempty"`
}

// SetDefinitionDefaults sets some default values for the given Definition.
func SetDefinitionDefaults(def *Definition) {
	// default to local arch
	if def.Image.Arch == "" {
		def.Image.Arch = runtime.GOARCH
	}

	// set default expiry of 30 days
	if def.Image.Expiry == "" {
		def.Image.Expiry = "30d"
	}

	// Set default variant
	if def.Image.Variant == "" {
		def.Image.Variant = "default"
	}

	if def.Source.Keyserver == "" {
		def.Source.Keyserver = "hkps.pool.sks-keyservers.net"
	}

	// If no Source.Variant is specified, use the one in Image.Variant.
	if def.Source.Variant == "" {
		def.Source.Variant = def.Image.Variant
	}
}

// ValidateDefinition validates the given Definition.
func ValidateDefinition(def Definition) error {
	if strings.TrimSpace(def.Image.Distribution) == "" {
		return errors.New("image.distribution may not be empty")
	}

	validDownloaders := []string{
		"alpinelinux-http",
		"archlinux-http",
		"centos-http",
		"debootstrap",
		"ubuntu-http",
	}
	if !shared.StringInSlice(strings.TrimSpace(def.Source.Downloader), validDownloaders) {
		return fmt.Errorf("source.downloader must be one of %v", validDownloaders)
	}

	validManagers := []string{
		"apk",
		"apt",
		"yum",
		"pacman",
	}
	if !shared.StringInSlice(strings.TrimSpace(def.Packages.Manager), validManagers) {
		return fmt.Errorf("packages.manager must be one of %v", validManagers)
	}

	return nil
}
