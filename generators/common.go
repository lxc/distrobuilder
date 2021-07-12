package generators

import (
	"go.uber.org/zap"

	"github.com/lxc/distrobuilder/shared"
)

type common struct {
	logger    *zap.SugaredLogger
	cacheDir  string
	sourceDir string
	defFile   shared.DefinitionFile
}

func (g *common) init(logger *zap.SugaredLogger, cacheDir string, sourceDir string, defFile shared.DefinitionFile) {
	g.logger = logger
	g.cacheDir = cacheDir
	g.sourceDir = sourceDir
	g.defFile = defFile
}
