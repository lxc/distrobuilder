package shared

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/lxd/shared"
	lxdarch "github.com/canonical/lxd/shared/osarch"
)

// ImageTarget represents the image target.
type ImageTarget int

const (
	// ImageTargetAll is used for all targets.
	ImageTargetAll ImageTarget = 1

	// ImageTargetContainer is used for container targets.
	ImageTargetContainer ImageTarget = 1 << 1

	// ImageTargetVM is used for VM targets.
	ImageTargetVM ImageTarget = 1 << 2

	// ImageTargetUndefined is used when no type has been specified.
	ImageTargetUndefined ImageTarget = 1 << 3
)

// DefinitionFilterType represents the filter type.
type DefinitionFilterType string

const (
	// DefinitionFilterTypeVM is used for VMs.
	DefinitionFilterTypeVM DefinitionFilterType = "vm"

	// DefinitionFilterTypeContainer is used for containers.
	DefinitionFilterTypeContainer DefinitionFilterType = "container"
)

// UnmarshalYAML validates the filter type.
func (d *DefinitionFilterType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var filterType string

	err := unmarshal(&filterType)
	if err != nil {
		return err
	}

	if DefinitionFilterType(filterType) != DefinitionFilterTypeContainer && DefinitionFilterType(filterType) != DefinitionFilterTypeVM {
		return fmt.Errorf("Invalid filter type %q", filterType)
	}

	*d = DefinitionFilterType(filterType)

	return nil
}

// Filter represents a filter.
type Filter interface {
	GetReleases() []string
	GetArchitectures() []string
	GetVariants() []string
	GetTypes() []DefinitionFilterType
}

// A DefinitionFilter defines filters for various actions.
type DefinitionFilter struct {
	Releases      []string               `yaml:"releases,omitempty"`
	Architectures []string               `yaml:"architectures,omitempty"`
	Variants      []string               `yaml:"variants,omitempty"`
	Types         []DefinitionFilterType `yaml:"types,omitempty"`
}

// GetReleases returns a list of releases.
func (d *DefinitionFilter) GetReleases() []string {
	return d.Releases
}

// GetArchitectures returns a list of architectures.
func (d *DefinitionFilter) GetArchitectures() []string {
	return d.Architectures
}

// GetVariants returns a list of variants.
func (d *DefinitionFilter) GetVariants() []string {
	return d.Variants
}

// GetTypes returns a list of types.
func (d *DefinitionFilter) GetTypes() []DefinitionFilterType {
	return d.Types
}

// A DefinitionPackagesSet is a set of packages which are to be installed
// or removed.
type DefinitionPackagesSet struct {
	DefinitionFilter `yaml:",inline"`
	Packages         []string `yaml:"packages"`
	Action           string   `yaml:"action"`
	Early            bool     `yaml:"early,omitempty"`
	Flags            []string `yaml:"flags,omitempty"`
}

// A DefinitionPackagesRepository contains data of a specific repository.
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
	CustomManager *DefinitionPackagesCustomManager `yaml:"custom_manager,omitempty"`
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

// A DefinitionSource specifies the download type and location.
type DefinitionSource struct {
	Downloader       string   `yaml:"downloader"`
	URL              string   `yaml:"url,omitempty"`
	Keys             []string `yaml:"keys,omitempty"`
	Keyserver        string   `yaml:"keyserver,omitempty"`
	Variant          string   `yaml:"variant,omitempty"`
	Suite            string   `yaml:"suite,omitempty"`
	SameAs           string   `yaml:"same_as,omitempty"`
	SkipVerification bool     `yaml:"skip_verification,omitempty"`
	Components       []string `yaml:"components,omitempty"`
}

// A DefinitionTargetLXCConfig represents the config part of the metadata.
type DefinitionTargetLXCConfig struct {
	DefinitionFilter `yaml:",inline"`
	Type             string `yaml:"type"`
	Before           uint   `yaml:"before,omitempty"`
	After            uint   `yaml:"after,omitempty"`
	Content          string `yaml:"content"`
}

// A DefinitionTargetLXC represents LXC specific files as part of the metadata.
type DefinitionTargetLXC struct {
	CreateMessage string                      `yaml:"create_message,omitempty"`
	Config        []DefinitionTargetLXCConfig `yaml:"config,omitempty"`
}

// DefinitionTargetLXDVM represents LXD VM specific options.
type DefinitionTargetLXDVM struct {
	Size       uint64 `yaml:"size,omitempty"`
	Filesystem string `yaml:"filesystem,omitempty"`
}

// DefinitionTargetLXD represents LXD specific options.
type DefinitionTargetLXD struct {
	VM DefinitionTargetLXDVM `yaml:"vm,omitempty"`
}

// A DefinitionTarget specifies target dependent files.
type DefinitionTarget struct {
	LXC  DefinitionTargetLXC  `yaml:"lxc,omitempty"`
	LXD  DefinitionTargetLXD  `yaml:"lxd,omitempty"`
	Type DefinitionFilterType // This field is internal only and used only for simplicity.
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
	Mode             string                 `yaml:"mode,omitempty"`
	GID              string                 `yaml:"gid,omitempty"`
	UID              string                 `yaml:"uid,omitempty"`
	Pongo            bool                   `yaml:"pongo,omitempty"`
	Source           string                 `yaml:"source,omitempty"`
}

// A DefinitionFileTemplate represents the settings used by generators.
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
	Pongo            bool   `yaml:"pongo,omitempty"`
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
		return fmt.Errorf("Failed to get field by tag: %w", err)
	}

	// Fail if the field cannot be set
	if !field.CanSet() {
		return fmt.Errorf("Cannot set value for %s", key)
	}

	switch field.Kind() {
	case reflect.Bool:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("Failed to parse bool %q: %w", value, err)
		}

		field.SetBool(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("Failed to parse int %q: %w", value, err)
		}

		field.SetInt(v)
	case reflect.String:
		field.SetString(value)
	case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("Failed to parse uint %q: %w", value, err)
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

	// Set default target type. This will only be overridden if building VMs for LXD.
	d.Targets.Type = DefinitionFilterTypeContainer
}

// Validate validates the Definition.
func (d *Definition) Validate() error {
	if strings.TrimSpace(d.Image.Distribution) == "" {
		return errors.New("image.distribution may not be empty")
	}

	validDownloaders := []string{
		"almalinux-http",
		"alpinelinux-http",
		"alt-http",
		"apertis-http",
		"archlinux-http",
		"busybox",
		"centos-http",
		"springdalelinux-http",
		"debootstrap",
		"fedora-http",
		"gentoo-http",
		"ubuntu-http",
		"sabayon-http",
		"docker-http",
		"oraclelinux-http",
		"openeuler-http",
		"opensuse-http",
		"openwrt-http",
		"plamolinux-http",
		"voidlinux-http",
		"funtoo-http",
		"rootfs-http",
		"rockylinux-http",
		"vyos-http",
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
			"luet",
		}

		if !shared.StringInSlice(strings.TrimSpace(d.Packages.Manager), validManagers) {
			return fmt.Errorf("packages.manager must be one of %v", validManagers)
		}

		if d.Packages.CustomManager != nil {
			return errors.New("cannot have both packages.manager and packages.custom_manager set")
		}
	} else {
		if d.Packages.CustomManager == nil {
			return errors.New("packages.manager or packages.custom_manager needs to be set")
		}

		if d.Packages.CustomManager.Clean.Command == "" {
			return errors.New("packages.custom_manager requires a clean command")
		}

		if d.Packages.CustomManager.Install.Command == "" {
			return errors.New("packages.custom_manager requires an install command")
		}

		if d.Packages.CustomManager.Remove.Command == "" {
			return errors.New("packages.custom_manager requires a remove command")
		}

		if d.Packages.CustomManager.Refresh.Command == "" {
			return errors.New("packages.custom_manager requires a refresh command")
		}

		if d.Packages.CustomManager.Update.Command == "" {
			return errors.New("packages.custom_manager requires an update command")
		}
	}

	validGenerators := []string{
		"dump",
		"copy",
		"template",
		"hostname",
		"hosts",
		"remove",
		"cloud-init",
		"lxd-agent",
		"fstab",
	}

	for _, file := range d.Files {
		if !shared.StringInSlice(strings.TrimSpace(file.Generator), validGenerators) {
			return fmt.Errorf("files.*.generator must be one of %v", validGenerators)
		}
	}

	validMappings := []string{
		"almalinux",
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
		return fmt.Errorf("Failed to get mapped architecture: %w", err)
	}

	d.Image.ArchitectureMapped = archMapped

	// Kernel architecture and personality
	archID, err := lxdarch.ArchitectureId(d.Image.Architecture)
	if err != nil {
		return fmt.Errorf("Failed to get architecture ID: %w", err)
	}

	archName, err := lxdarch.ArchitectureName(archID)
	if err != nil {
		return fmt.Errorf("Failed to get architecture name: %w", err)
	}

	d.Image.ArchitectureKernel = archName

	archPersonality, err := lxdarch.ArchitecturePersonality(archID)
	if err != nil {
		return fmt.Errorf("Failed to get architecture personality: %w", err)
	}

	d.Image.ArchitecturePersonality = archPersonality

	return nil
}

// GetRunnableActions returns a list of actions depending on the trigger
// and releases.
func (d *Definition) GetRunnableActions(trigger string, imageTarget ImageTarget) []DefinitionAction {
	out := []DefinitionAction{}

	for _, action := range d.Actions {
		if action.Trigger != trigger {
			continue
		}

		out = append(out, action)
	}

	return out
}

// GetEarlyPackages returns a list of packages which are to be installed or removed earlier than the actual package handling
// Also removes them from the package set so they aren't attempted to be re-installed again as normal packages.
func (d *Definition) GetEarlyPackages(action string) []string {
	var early []string

	normal := []DefinitionPackagesSet{}

	for _, set := range d.Packages.Sets {
		if set.Early && set.Action == action {
			early = append(early, set.Packages...)
		} else {
			normal = append(normal, set)
		}
	}

	d.Packages.Sets = normal

	return early
}

func (d *Definition) getMappedArchitecture() (string, error) {
	var arch string

	if d.Mappings.ArchitectureMap != "" {
		// Translate the architecture using the requested map
		var err error
		arch, err = GetArch(d.Mappings.ArchitectureMap, d.Image.Architecture)
		if err != nil {
			return "", fmt.Errorf("Failed to translate the architecture name: %w", err)
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

// ApplyFilter returns true if the filter matches.
func (d *Definition) applyFilter(filter Filter, acceptedImageTargets ImageTarget) bool {
	if len(filter.GetReleases()) > 0 && !shared.StringInSlice(d.Image.Release, filter.GetReleases()) {
		return false
	}

	if len(filter.GetArchitectures()) > 0 && !shared.StringInSlice(d.Image.ArchitectureMapped, filter.GetArchitectures()) {
		return false
	}

	if len(filter.GetVariants()) > 0 && !shared.StringInSlice(d.Image.Variant, filter.GetVariants()) {
		return false
	}

	types := filter.GetTypes()

	if (acceptedImageTargets == 0 || acceptedImageTargets&ImageTargetUndefined > 0) && len(types) == 0 {
		return true
	}

	hasTargetType := func(targetType DefinitionFilterType) bool {
		for _, t := range types {
			if t == targetType {
				return true
			}
		}

		return false
	}

	if acceptedImageTargets&ImageTargetAll > 0 {
		if len(types) == 2 && hasTargetType(d.Targets.Type) {
			return true
		}
	}

	if acceptedImageTargets&ImageTargetContainer > 0 {
		if d.Targets.Type == DefinitionFilterTypeContainer && hasTargetType(d.Targets.Type) {
			return true
		}
	}

	if acceptedImageTargets&ImageTargetVM > 0 {
		if d.Targets.Type == DefinitionFilterTypeVM && hasTargetType(d.Targets.Type) {
			return true
		}
	}

	return false
}

// ApplyFilters removes those parts of the definition which are excluded by the filters.
func (d *Definition) ApplyFilters(imageTargets ImageTarget) {
	newDefinition := Definition{
		Actions:     []DefinitionAction{},
		Environment: d.Environment,
		Files:       []DefinitionFile{},
		Image:       d.Image,
		Mappings:    d.Mappings,
		Packages:    d.Packages,
		Source:      d.Source,
		Targets:     d.Targets,
	}

	// Filter files
	for _, file := range d.Files {
		if !d.applyFilter(&file, imageTargets) {
			continue
		}

		newDefinition.Files = append(newDefinition.Files, file)
	}

	// Filter repositories
	newDefinition.Packages.Repositories = []DefinitionPackagesRepository{}

	for _, repo := range d.Packages.Repositories {
		if !d.applyFilter(&repo, imageTargets) {
			continue
		}

		newDefinition.Packages.Repositories = append(newDefinition.Packages.Repositories, repo)
	}

	// Filter package sets
	newDefinition.Packages.Sets = []DefinitionPackagesSet{}

	for _, set := range d.Packages.Sets {
		if !d.applyFilter(&set, imageTargets) {
			continue
		}

		newDefinition.Packages.Sets = append(newDefinition.Packages.Sets, set)
	}

	// Filter actions
	for _, action := range d.Actions {
		if !d.applyFilter(&action, imageTargets) {
			continue
		}

		newDefinition.Actions = append(newDefinition.Actions, action)
	}

	// Filter targets
	newDefinition.Targets.LXC.Config = []DefinitionTargetLXCConfig{}

	for _, config := range d.Targets.LXC.Config {
		// Always add ImageTargetUndefined when handling LXC targets.
		// These are only applied when running {build,pack}-lxc, and shouldn't require
		// explicitly setting the "container" type filter.
		if !d.applyFilter(&config, ImageTargetUndefined|imageTargets) {
			continue
		}

		newDefinition.Targets.LXC.Config = append(newDefinition.Targets.LXC.Config, config)
	}

	*d = newDefinition
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
