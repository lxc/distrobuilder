package shared

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

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
	Architecture string `yaml:"arch,omitempty"`
	Expiry       string `yaml:"expiry,omitempty"`
	Variant      string `yaml:"variant,omitempty"`
	Name         string `yaml:"name,omitempty"`
	Serial       string `yaml:"serial,omitempty"`
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
	if def.Image.Architecture == "" {
		def.Image.Architecture = runtime.GOARCH
	}

	// set default expiry of 30 days
	if def.Image.Expiry == "" {
		def.Image.Expiry = "30d"
	}

	// Set default serial number
	if def.Image.Serial == "" {
		def.Image.Serial = time.Now().UTC().Format("20060102_1504")
	}

	// Set default variant
	if def.Image.Variant == "" {
		def.Image.Variant = "default"
	}

	// Set default keyserver
	if def.Source.Keyserver == "" {
		def.Source.Keyserver = "hkps.pool.sks-keyservers.net"
	}

	// Set default name and description templates
	if def.Image.Name == "" {
		def.Image.Name = "{{ image.Distribution }}-{{ image.Release }}-{{ image.Architecture }}-{{ image.Variant }}-{{ image.Serial }}"
	}

	if def.Image.Description == "" {
		def.Image.Description = "{{ image.Distribution|capfirst }} {{ image.Release }} {{ image.Architecture }}{% if image.Variant != \"default\" %} ({{ image.Variant }}){% endif %} ({{ image.Serial }})"
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

	validGenerators := []string{
		"dump",
		"hostname",
		"hosts",
		"remove",
		"upstart-tty",
	}

	for _, file := range def.Files {
		if !shared.StringInSlice(strings.TrimSpace(file.Generator), validGenerators) {
			return fmt.Errorf("files.*.generator must be one of %v", validGenerators)
		}
	}

	validMappings := []string{
		"alpinelinux",
		"centos",
		"debian",
	}

	architectureMap := strings.TrimSpace(def.Mappings.ArchitectureMap)
	if architectureMap != "" {
		if !shared.StringInSlice(architectureMap, validMappings) {
			return fmt.Errorf("mappings.architecture_map must be one of %v", validMappings)
		}
	}

	return nil
}
