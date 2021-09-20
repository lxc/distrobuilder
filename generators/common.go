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

func (g *common) init(logger *zap.SugaredLogger, cacheDir string, sourceDir string, defFile shared.DefinitionFile, def shared.Definition) {
	g.logger = logger
	g.cacheDir = cacheDir
	g.sourceDir = sourceDir
	g.defFile = defFile

	render := func(val string) string {
		if !defFile.Pongo {
			return val
		}

		out, err := shared.RenderTemplate(val, def)
		if err != nil {
			logger.Warnw("Failed to render template", "err", err)
			return val
		}

		return out
	}

	if defFile.Pongo {
		g.defFile.Content = render(g.defFile.Content)
		g.defFile.Path = render(g.defFile.Path)
		g.defFile.Source = render(g.defFile.Source)
	}
}
