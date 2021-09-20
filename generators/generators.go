package generators

import (
	"errors"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
	"go.uber.org/zap"
)

// ErrNotSupported returns a "Not supported" error
var ErrNotSupported = errors.New("Not supported")

// ErrUnknownGenerator represents the unknown generator error
var ErrUnknownGenerator = errors.New("Unknown generator")

type generator interface {
	init(logger *zap.SugaredLogger, cacheDir string, sourceDir string, defFile shared.DefinitionFile, def shared.Definition)

	Generator
}

// Generator interface.
type Generator interface {
	RunLXC(*image.LXCImage, shared.DefinitionTargetLXC) error
	RunLXD(*image.LXDImage, shared.DefinitionTargetLXD) error
	Run() error
}

var generators = map[string]func() generator{
	"cloud-init": func() generator { return &cloudInit{} },
	"copy":       func() generator { return &copy{} },
	"dump":       func() generator { return &dump{} },
	"fstab":      func() generator { return &fstab{} },
	"hostname":   func() generator { return &hostname{} },
	"hosts":      func() generator { return &hosts{} },
	"lxd-agent":  func() generator { return &lxdAgent{} },
	"remove":     func() generator { return &remove{} },
	"template":   func() generator { return &template{} },
}

// Load loads and initializes a generator.
func Load(generatorName string, logger *zap.SugaredLogger, cacheDir string, sourceDir string, defFile shared.DefinitionFile, def shared.Definition) (Generator, error) {
	df, ok := generators[generatorName]
	if !ok {
		return nil, ErrUnknownGenerator
	}

	d := df()

	d.init(logger, cacheDir, sourceDir, defFile, def)

	return d, nil
}
