package sources

import (
	"github.com/lxc/distrobuilder/shared"
	"go.uber.org/zap"
)

type common struct {
	logger     *zap.SugaredLogger
	definition shared.Definition
	rootfsDir  string
}

func (s *common) init(logger *zap.SugaredLogger, definition shared.Definition, rootfsDir string) {
	s.logger = logger
	s.definition = definition
	s.rootfsDir = rootfsDir
}
