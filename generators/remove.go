package generators

import (
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

type remove struct {
	common
}

// RunLXC removes a path.
func (g *remove) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {
	return g.Run()
}

// RunIncus removes a path.
func (g *remove) RunIncus(img *image.IncusImage, target shared.DefinitionTargetIncus) error {
	return g.Run()
}

// Run removes a path.
func (g *remove) Run() error {
	return os.RemoveAll(filepath.Join(g.sourceDir, g.defFile.Path))
}
