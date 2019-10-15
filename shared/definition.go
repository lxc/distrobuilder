package shared

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/lxc/lxd/shared"
	lxdarch "github.com/lxc/lxd/shared/osarch"
)

// A DefinitionFilter defines filters for various actions.
type DefinitionFilter struct {
	Releases      []string `yaml:"releases,omitempty"`
	Architectures []string `yaml:"architectures,omitempty"`
	Variants      []string `yaml:"variants,omitempty"`
}

// A DefinitionPackagesSet is a set of packages which are to be installed
// or removed.
type DefinitionPackagesSet struct {
	DefinitionFilter `yaml:",inline"`
	Packages         []string `yaml:"packages"`
	Action           string   `yaml:"action"`
	Early            bool     `yaml:"early,omitempty"`
}

// A DefinitionPackagesRepository contains data of a specific repository
type DefinitionPackagesRepository struct {
	DefinitionFilter `yaml:",inline"`
	Name             string `yaml:"name"`           // Name of the repository
	URL              string `yaml:"url"`            // URL (may differ based on manager)
	Type             string `yaml:"type,omitempty"` // For distros that have more than one repository manager
	Key              string `yaml:"key,omitempty"`  // GPG armored keyring
}

// CustomManagerCmd represents a command for a custom manager.
type CustomManagerCmd struct {
	Command string   `yaml:"cmd"`
	Flags   []string `yaml:"flags,omitempty"`
}

// DefinitionPackagesCustomManager represents a custom package manager.
type DefinitionPackagesCustomManager struct {
	Clean   CustomManagerCmd `yaml:"clean"`
	Install CustomManagerCmd `yaml:"install"`
	Remove  CustomManagerCmd `yaml:"remove"`
	Refresh CustomManagerCmd `yaml:"refresh"`
	Update  CustomManagerCmd `yaml:"update"`
	Flags   []string         `yaml:"flags,omitempty"`
}

// A DefinitionPackages represents a package handler.
type DefinitionPackages struct {
	Manager       string                           `yaml:"manager,omitempty"`
	CustomManager *DefinitionPackagesCustomManager `yaml:"custom-manager,omitempty"`
	Update        bool                             `yaml:"update,omitempty"`
	Cleanup       bool                             `yaml:"cleanup,omitempty"`
	Sets          []DefinitionPackagesSet          `yaml:"sets,omitempty"`
	Repositories  []DefinitionPackagesRepository   `yaml:"repositories,omitempty"`
}

// A DefinitionImage represents the image.
type DefinitionImage struct {
	Description  string `yaml:"description"`
	Distribution string `yaml:"distribution"`
	Release      string `yaml:"release,omitempty"`
	Architecture string `yaml:"architecture,omitempty"`
	Expiry       string `yaml:"expiry,omitempty"`
	Variant      string `yaml:"variant,omitempty"`
	Name         string `yaml:"name,omitempty"`
	Serial       string `yaml:"serial,omitempty"`

	// Internal fields (YAML input ignored)
	ArchitectureMapped      string `yaml:"architecture_mapped,omitempty"`
	ArchitectureKernel      string `yaml:"architecture_kernel,omitempty"`
	ArchitecturePersonality string `yaml:"architecture_personality,omitempty"`
}

// A DefinitionSource specifies the download type and location
type DefinitionSource struct {
	Downloader       string   `yaml:"downloader"`
	URL              string   `yaml:"url,omitempty"`
	Keys             []string `yaml:"keys,omitempty"`
	Keyserver        string   `yaml:"keyserver,omitempty"`
	Variant          string   `yaml:"variant,omitempty"`
	Suite            string   `yaml:"suite,omitempty"`
	SameAs           string   `yaml:"same_as,omitempty"`
	SkipVerification bool     `yaml:"skip_verification,omitempty"`
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
	DefinitionFilter `yaml:",inline"`
	Generator        string                 `yaml:"generator"`
	Path             string                 `yaml:"path,omitempty"`
	Content          string                 `yaml:"content,omitempty"`
	Name             string                 `yaml:"name,omitempty"`
	Template         DefinitionFileTemplate `yaml:"template,omitempty"`
	Templated        bool                   `yaml:"templated,omitempty"`
}

// A DefinitionFileTemplate represents the settings used by generators
type DefinitionFileTemplate struct {
	Properties map[string]string `yaml:"properties,omitempty"`
	When       []string          `yaml:"when,omitempty"`
}

// A DefinitionAction specifies a custom action (script) which is to be run after
// a certain action.
type DefinitionAction struct {
	DefinitionFilter `yaml:",inline"`
	Trigger          string `yaml:"trigger"`
	Action           string `yaml:"action"`
}

// DefinitionMappings defines custom mappings.
type DefinitionMappings struct {
	Architectures   map[string]string `yaml:"architectures,omitempty"`
	ArchitectureMap string            `yaml:"architecture_map,omitempty"`
}

// DefinitionEnvVars defines custom environment variables.
type DefinitionEnvVars struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

// DefinitionEnv represents the config part of the environment section.
type DefinitionEnv struct {
	ClearDefaults bool                `yaml:"clear_defaults,omitempty"`
	EnvVariables  []DefinitionEnvVars `yaml:"variables,omitempty"`
}

// A Definition a definition.
type Definition struct {
	Image       DefinitionImage    `yaml:"image"`
	Source      DefinitionSource   `yaml:"source"`
	Targets     DefinitionTarget   `yaml:"targets,omitempty"`
	Files       []DefinitionFile   `yaml:"files,omitempty"`
	Packages    DefinitionPackages `yaml:"packages,omitempty"`
	Actions     []DefinitionAction `yaml:"actions,omitempty"`
	Mappings    DefinitionMappings `yaml:"mappings,omitempty"`
	Environment DefinitionEnv      `yaml:"environment,omitempty"`
}

// SetValue writes the provided value to a field represented by the yaml tag 'key'.
func (d *Definition) SetValue(key string, value string) error {
	// Walk through the definition and find the field with the given key
	field, err := getFieldByTag(reflect.ValueOf(d).Elem(), reflect.TypeOf(d).Elem(), key)
	if err != nil {
		return err
	}

	// Fail if the field cannot be set
	if !field.CanSet() {
		return fmt.Errorf("Cannot set value for %s", key)
	}

	switch field.Kind() {
	case reflect.Bool:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(v)
	case reflect.String:
		field.SetString(value)
	case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(v)
	default:
		return fmt.Errorf("Unsupported type '%s'", field.Kind())
	}

	return nil
}

// SetDefaults sets some default values.
func (d *Definition) SetDefaults() {
	// default to local arch
	if d.Image.Architecture == "" {
		uname, _ := shared.Uname()
		d.Image.Architecture = uname.Machine
	}

	// set default expiry of 30 days
	if d.Image.Expiry == "" {
		d.Image.Expiry = "30d"
	}

	// Set default serial number
	if d.Image.Serial == "" {
		d.Image.Serial = time.Now().UTC().Format("20060102_1504")
	}

	// Set default variant
	if d.Image.Variant == "" {
		d.Image.Variant = "default"
	}

	// Set default keyserver
	if d.Source.Keyserver == "" {
		d.Source.Keyserver = "hkps.pool.sks-keyservers.net"
	}

	// Set default name and description templates
	if d.Image.Name == "" {
		d.Image.Name = "{{ image.distribution }}-{{ image.release }}-{{ image.architecture_mapped }}-{{ image.variant }}-{{ image.serial }}"
	}

	if d.Image.Description == "" {
		d.Image.Description = "{{ image.distribution|capfirst }} {{ image.release }} {{ image.architecture_mapped }}{% if image.variant != \"default\" %} ({{ image.variant }}){% endif %} ({{ image.serial }})"
	}
}

// Validate validates the Definition.
func (d *Definition) Validate() error {
	if strings.TrimSpace(d.Image.Distribution) == "" {
		return errors.New("image.distribution may not be empty")
	}

	validDownloaders := []string{
		"alpinelinux-http",
		"alt-http",
		"apertis-http",
		"archlinux-http",
		"centos-http",
		"debootstrap",
		"fedora-http",
		"gentoo-http",
		"ubuntu-http",
		"sabayon-http",
		"docker-http",
		"oraclelinux-http",
		"opensuse-http",
		"openwrt-http",
		"plamolinux-http",
		"voidlinux-http",
		"funtoo-http",
	}
	if !shared.StringInSlice(strings.TrimSpace(d.Source.Downloader), validDownloaders) {
		return fmt.Errorf("source.downloader must be one of %v", validDownloaders)
	}

	if d.Packages.Manager != "" {
		validManagers := []string{
			"apk",
			"apt",
			"dnf",
			"egoportage",
			"opkg",
			"pacman",
			"portage",
			"yum",
			"equo",
			"xbps",
			"zypper",
		}
		if !shared.StringInSlice(strings.TrimSpace(d.Packages.Manager), validManagers) {
			return fmt.Errorf("packages.manager must be one of %v", validManagers)
		}

		if d.Packages.CustomManager != nil {
			return fmt.Errorf("cannot have both packages.manager and packages.custom-manager set")
		}
	} else {
		if d.Packages.CustomManager == nil {
			return fmt.Errorf("packages.manager or packages.custom-manager needs to be set")
		}

		if d.Packages.CustomManager.Clean.Command == "" {
			return fmt.Errorf("packages.custom-manager requires a clean command")
		}

		if d.Packages.CustomManager.Install.Command == "" {
			return fmt.Errorf("packages.custom-manager requires an install command")
		}

		if d.Packages.CustomManager.Remove.Command == "" {
			return fmt.Errorf("packages.custom-manager requires a remove command")
		}

		if d.Packages.CustomManager.Refresh.Command == "" {
			return fmt.Errorf("packages.custom-manager requires a refresh command")
		}

		if d.Packages.CustomManager.Update.Command == "" {
			return fmt.Errorf("packages.custom-manager requires an update command")
		}
	}

	validGenerators := []string{
		"dump",
		"template",
		"hostname",
		"hosts",
		"remove",
		"upstart-tty",
		"cloud-init",
	}

	for _, file := range d.Files {
		if !shared.StringInSlice(strings.TrimSpace(file.Generator), validGenerators) {
			return fmt.Errorf("files.*.generator must be one of %v", validGenerators)
		}
	}

	validMappings := []string{
		"alpinelinux",
		"altlinux",
		"archlinux",
		"centos",
		"debian",
		"gentoo",
		"plamolinux",
		"voidlinux",
		"funtoo",
	}

	architectureMap := strings.TrimSpace(d.Mappings.ArchitectureMap)
	if architectureMap != "" {
		if !shared.StringInSlice(architectureMap, validMappings) {
			return fmt.Errorf("mappings.architecture_map must be one of %v", validMappings)
		}
	}

	validTriggers := []string{
		"post-files",
		"post-packages",
		"post-unpack",
		"post-update",
	}

	for _, action := range d.Actions {
		if !shared.StringInSlice(action.Trigger, validTriggers) {
			return fmt.Errorf("actions.*.trigger must be one of %v", validTriggers)
		}
	}

	validPackageActions := []string{
		"install",
		"remove",
	}

	for _, set := range d.Packages.Sets {
		if !shared.StringInSlice(set.Action, validPackageActions) {
			return fmt.Errorf("packages.*.set.*.action must be one of %v", validPackageActions)
		}
	}

	// Mapped architecture (distro name)
	archMapped, err := d.getMappedArchitecture()
	if err != nil {
		return err
	}

	d.Image.ArchitectureMapped = archMapped

	// Kernel architecture and personality
	archID, err := lxdarch.ArchitectureId(d.Image.Architecture)
	if err != nil {
		return err
	}

	archName, err := lxdarch.ArchitectureName(archID)
	if err != nil {
		return err
	}

	d.Image.ArchitectureKernel = archName

	archPersonality, err := lxdarch.ArchitecturePersonality(archID)
	if err != nil {
		return err
	}

	d.Image.ArchitecturePersonality = archPersonality

	return nil
}

// GetRunnableActions returns a list of actions depending on the trigger
// and releases.
func (d *Definition) GetRunnableActions(trigger string) []DefinitionAction {
	out := []DefinitionAction{}

	for _, action := range d.Actions {
		if action.Trigger != trigger {
			continue
		}

		if len(action.Releases) > 0 && !shared.StringInSlice(d.Image.Release, action.Releases) {
			continue
		}

		if len(action.Architectures) > 0 && !shared.StringInSlice(d.Image.ArchitectureMapped, action.Architectures) {
			continue
		}

		if len(action.Variants) > 0 && !shared.StringInSlice(d.Image.Variant, action.Variants) {
			continue
		}

		out = append(out, action)
	}

	return out
}

// GetEarlyPackages returns a list of packages which are to be installed or removed earlier than the actual package handling.
func (d *Definition) GetEarlyPackages(action string) []string {
	var out []string

	for _, set := range d.Packages.Sets {
		if set.Action != action || !set.Early {
			continue
		}

		if len(set.Releases) > 0 && !shared.StringInSlice(d.Image.Release, set.Releases) {
			continue
		}

		if len(set.Architectures) > 0 && !shared.StringInSlice(d.Image.ArchitectureMapped, set.Architectures) {
			continue
		}

		if len(set.Variants) > 0 && !shared.StringInSlice(d.Image.Variant, set.Variants) {
			continue
		}

		out = append(out, set.Packages...)
	}

	return out
}

func (d *Definition) getMappedArchitecture() (string, error) {
	var arch string

	if d.Mappings.ArchitectureMap != "" {
		// Translate the architecture using the requested map
		var err error
		arch, err = GetArch(d.Mappings.ArchitectureMap, d.Image.Architecture)
		if err != nil {
			return "", fmt.Errorf("Failed to translate the architecture name: %s", err)
		}
	} else if len(d.Mappings.Architectures) > 0 {
		// Translate the architecture using a user specified mapping
		var ok bool
		arch, ok = d.Mappings.Architectures[d.Image.Architecture]
		if !ok {
			// If no mapping exists, it means it doesn't need translating
			arch = d.Image.Architecture
		}
	} else {
		// No map or mappings provided, just go with it as it is
		arch = d.Image.Architecture
	}

	return arch, nil
}

func getFieldByTag(v reflect.Value, t reflect.Type, tag string) (reflect.Value, error) {
	parts := strings.SplitN(tag, ".", 2)

	if t.Kind() == reflect.Slice {
		// Get index, e.g. '0' from tag 'foo.0'
		value, err := strconv.Atoi(parts[0])
		if err != nil {
			return reflect.Value{}, err
		}

		if t.Elem().Kind() == reflect.Struct {
			// Make sure we are in range, otherwise return error
			if value < 0 || value >= v.Len() {
				return reflect.Value{}, errors.New("Index out of range")
			}
			return getFieldByTag(v.Index(value), t.Elem(), parts[1])
		}

		// Primitive type
		return v.Index(value), nil
	}

	if t.Kind() == reflect.Struct {
		// Find struct field with correct tag
		for i := 0; i < t.NumField(); i++ {
			value := t.Field(i).Tag.Get("yaml")
			if value != "" && strings.Split(value, ",")[0] == parts[0] {
				if len(parts) == 1 {
					return v.Field(i), nil
				}
				return getFieldByTag(v.Field(i), t.Field(i).Type, parts[1])
			}
		}
	}

	// Return its value if it's a primitive type
	return v, nil
}
