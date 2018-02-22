package image

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/lxd/shared/osarch"
	pongo2 "gopkg.in/flosch/pongo2.v3"
	yaml "gopkg.in/yaml.v2"
)

// A LXDMetadataTemplate represents template information.
type LXDMetadataTemplate struct {
	Template   string                       `yaml:"template"`
	When       []string                     `yaml:"when"`
	Trigger    string                       `yaml:"trigger,omitempty"`
	Path       string                       `yaml:"path,omitempty"`
	Container  map[string]string            `yaml:"container,omitempty"`
	Config     map[string]string            `yaml:"config,omitempty"`
	Devices    map[string]map[string]string `yaml:"devices,omitempty"`
	Properties map[string]string            `yaml:"properties,omitempty"`
	CreateOnly bool                         `yaml:"create_only,omitempty"`
}

// A LXDMetadataProperties represents properties of the LXD image.
type LXDMetadataProperties struct {
	Architecture string `yaml:"architecture"`
	Description  string `yaml:"description"`
	OS           string `yaml:"os"`
	Release      string `yaml:"release"`
	Variant      string `yaml:"variant,omitempty"`
	Name         string `yaml:"name,omitempty"`
}

// A LXDMetadata represents meta information about the LXD image.
type LXDMetadata struct {
	Architecture string                         `yaml:"architecture"`
	CreationDate int64                          `yaml:"creation_date"`
	Properties   LXDMetadataProperties          `yaml:"properties,omitempty"`
	Templates    map[string]LXDMetadataTemplate `yaml:"templates,omitempty"`
}

// A LXDImage represents a LXD image.
type LXDImage struct {
	cacheDir     string
	creationDate time.Time
	Metadata     LXDMetadata
	definition   shared.DefinitionImage
}

// NewLXDImage returns a LXDImage.
func NewLXDImage(cacheDir string, imageDef shared.DefinitionImage) *LXDImage {
	return &LXDImage{
		cacheDir,
		time.Now(),
		LXDMetadata{
			Templates: make(map[string]LXDMetadataTemplate),
		},
		imageDef,
	}
}

// Build creates a LXD image.
func (l *LXDImage) Build(unified bool) error {
	err := l.createMetadata()
	if err != nil {
		return nil
	}

	file, err := os.Create(filepath.Join(l.cacheDir, "metadata.yaml"))
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := yaml.Marshal(l.Metadata)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("Failed to write metadata: %s", err)
	}

	if unified {
		var fname string
		paths := []string{"rootfs", "templates", "metadata.yaml"}

		if l.definition.Name != "" {
			fname, _ = l.renderTemplate(l.definition.Name)
		} else {
			fname = "lxd"
		}

		err = shared.Pack(fmt.Sprintf("%s.tar.xz", fname), l.cacheDir, paths...)
		if err != nil {
			return err
		}
	} else {
		err = shared.RunCommand("mksquashfs", filepath.Join(l.cacheDir, "rootfs"),
			"rootfs.squashfs", "-noappend")
		if err != nil {
			return err
		}

		err = shared.Pack("lxd.tar.xz", l.cacheDir, "templates", "metadata.yaml")
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *LXDImage) createMetadata() error {
	var err error

	// Get the arch ID of the provided architecture.
	ID, err := osarch.ArchitectureId(l.definition.Arch)
	if err != nil {
		return err
	}

	// Get the "proper" name of the architecture.
	arch, err := osarch.ArchitectureName(ID)
	if err != nil {
		return err
	}

	l.Metadata.Architecture = arch
	l.Metadata.CreationDate = l.creationDate.Unix()
	l.Metadata.Properties = LXDMetadataProperties{
		Architecture: arch,
		OS:           l.definition.Distribution,
		Release:      l.definition.Release,
	}

	l.Metadata.Properties.Description, err = l.renderTemplate(l.definition.Description)
	if err != err {
		return nil
	}

	l.Metadata.Properties.Name, err = l.renderTemplate(l.definition.Name)
	if err != nil {
		return err
	}

	return err
}

func (l *LXDImage) renderTemplate(template string) (string, error) {
	var (
		err error
		ret string
	)

	ctx := pongo2.Context{
		"arch":          l.definition.Arch,
		"os":            l.definition.Distribution,
		"release":       l.definition.Release,
		"variant":       l.definition.Variant,
		"creation_date": l.creationDate.Format("20060201_1504"),
	}

	tpl, err := pongo2.FromString(template)
	if err != nil {
		return ret, err
	}

	ret, err = tpl.Execute(ctx)
	if err != nil {
		return ret, err
	}

	return ret, err
}
