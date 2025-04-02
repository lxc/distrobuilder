package managers

import (
	"errors"

	"github.com/lxc/distrobuilder/shared"
)

type equo struct {
	common
}

func (m *equo) load() error {
	m.commands = managerCommands{
		clean:   "equo",
		install: "equo",
		refresh: "equo",
		remove:  "equo",
		update:  "equo",
	}

	m.flags = managerFlags{
		global: []string{},
		clean: []string{
			"cleanup",
		},
		install: []string{
			"install",
		},
		remove: []string{
			"remove",
		},
		refresh: []string{
			"update",
		},
		update: []string{
			"upgrade",
		},
	}

	return nil
}

func (m *equo) manageRepository(repoAction shared.DefinitionPackagesRepository) error {
	switch repoAction.Type {
	case "", "equo":
		return m.equoRepoCaller(repoAction)
	case "enman":
		return m.enmanRepoCaller(repoAction)
	}

	return errors.New("Invalid repository Type")
}

func (m *equo) enmanRepoCaller(repo shared.DefinitionPackagesRepository) error {
	args := []string{
		"add",
	}

	if repo.Name == "" && repo.URL == "" {
		return errors.New("Missing both repository url and repository name")
	}

	if repo.URL != "" {
		args = append(args, repo.URL)
	} else {
		args = append(args, repo.Name)
	}

	return shared.RunCommand(m.ctx, nil, nil, "enman", args...)
}

func (m *equo) equoRepoCaller(repo shared.DefinitionPackagesRepository) error {
	if repo.Name == "" {
		return errors.New("Invalid repository name")
	}

	if repo.URL == "" {
		return errors.New("Invalid repository url")
	}

	return shared.RunCommand(m.ctx, nil, nil, "equo", "repo", "add", "--repo", repo.URL, "--pkg", repo.URL,
		repo.Name)
}
