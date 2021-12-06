package generators

import (
	"github.com/lxc/distrobuilder/shared"
	"github.com/sirupsen/logrus"
)

type common struct {
	logger    *logrus.Logger
	cacheDir  string
	sourceDir string
	defFile   shared.DefinitionFile
}

func (g *common) init(logger *logrus.Logger, cacheDir string, sourceDir string, defFile shared.DefinitionFile, def shared.Definition) {
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
			logger.WithField("err", err).Warn("Failed to render template")
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
