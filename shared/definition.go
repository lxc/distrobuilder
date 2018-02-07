package shared

import "runtime"

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
	Release      string `yaml:"release"`
	Arch         string `yaml:"arch,omitempty"`
	Expiry       string `yaml:"expiry,omitempty"`
	Variant      string `yaml:"variant,omitempty"`
	Name         string `yaml:"name,omitempty"`
}

// A DefinitionSource specifies the download type and location
type DefinitionSource struct {
	Downloader string `yaml:"downloader"`
	URL        string `yaml:"url"`
}

// A DefinitionTargetLXC represents LXC specific files as part of the metadata.
type DefinitionTargetLXC struct {
	CreateMessage string `yaml:"create-message,omitempty"`
	Config        string `yaml:"config,omitempty"`
	ConfigUser    string `yaml:"config-user,omitempty"`
}

// A DefinitionTarget specifies target dependent files.
type DefinitionTarget struct {
	LXC DefinitionTargetLXC `yaml:"lxc,omitempty"`
}

// A DefinitionFile represents a file which is to be created inside to chroot.
type DefinitionFile struct {
	Generator string   `yaml:"generator"`
	Path      string   `yaml:"path,omitempty"`
	Releases  []string `yaml:"releases,omitempty"`
}

// DefinitionActions specifies custom actions (scripts) which are to be run after
// certain actions.
type DefinitionActions struct {
	PostUnpack   string `yaml:"post-unpack,omitempty"`
	PostUpdate   string `yaml:"post-update,omitempty"`
	PostPackages string `yaml:"post-packages,omitempty"`
	PostFiles    string `yaml:"post-files,omitempty"`
}

// A Definition a definition.
type Definition struct {
	Image    DefinitionImage    `yaml:"image"`
	Source   DefinitionSource   `yaml:"source"`
	Targets  DefinitionTarget   `yaml:"targets,omitempty"`
	Files    []DefinitionFile   `yaml:"files,omitempty"`
	Packages DefinitionPackages `yaml:"packages,omitempty"`
	Actions  DefinitionActions  `yaml:"actions,omitempty"`
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
}
