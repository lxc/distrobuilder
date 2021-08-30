package managers

import (
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/lxc/distrobuilder/shared"
)

// ErrUnknownManager represents the unknown manager error
var ErrUnknownManager = errors.New("Unknown manager")

// managerFlags represents flags for all subcommands of a package manager.
type managerFlags struct {
	global  []string
	install []string
	remove  []string
	clean   []string
	update  []string
	refresh []string
}

// managerHooks represents custom hooks.
type managerHooks struct {
	clean      func() error
	preRefresh func() error
}

// managerCommands represents all commands.
type managerCommands struct {
	clean   string
	install string
	refresh string
	remove  string
	update  string
}

// Manager represents a package manager.
type Manager struct {
	mgr manager
	def shared.Definition
}

type manager interface {
	init(logger *zap.SugaredLogger, definition shared.Definition)
	load() error
	manageRepository(repo shared.DefinitionPackagesRepository) error
	install(pkgs, flags []string) error
	remove(pkgs, flags []string) error
	clean() error
	refresh() error
	update() error
}

var managers = map[string]func() manager{
	"":           func() manager { return &custom{} },
	"apk":        func() manager { return &apk{} },
	"apt":        func() manager { return &apt{} },
	"dnf":        func() manager { return &dnf{} },
	"egoportage": func() manager { return &egoportage{} },
	"equo":       func() manager { return &equo{} },
	"luet":       func() manager { return &luet{} },
	"opkg":       func() manager { return &opkg{} },
	"pacman":     func() manager { return &pacman{} },
	"portage":    func() manager { return &portage{} },
	"xbps":       func() manager { return &xbps{} },
	"yum":        func() manager { return &yum{} },
	"zypper":     func() manager { return &zypper{} },
}

// Load loads and initializes a downloader.
func Load(managerName string, logger *zap.SugaredLogger, definition shared.Definition) (*Manager, error) {
	df, ok := managers[managerName]
	if !ok {
		return nil, ErrUnknownManager
	}

	d := df()

	d.init(logger, definition)

	err := d.load()
	if err != nil {
		return nil, errors.WithMessagef(err, "Failed to load manager %q", managerName)
	}

	return &Manager{def: definition, mgr: d}, nil
}

// ManagePackages manages packages.
func (m *Manager) ManagePackages(imageTarget shared.ImageTarget) error {
	var validSets []shared.DefinitionPackagesSet

	for _, set := range m.def.Packages.Sets {
		if !shared.ApplyFilter(&set, m.def.Image.Release, m.def.Image.ArchitectureMapped, m.def.Image.Variant, m.def.Targets.Type, imageTarget) {
			continue
		}

		validSets = append(validSets, set)
	}

	// If there's nothing to install or remove, and no updates need to be performed,
	// we can exit here.
	if len(validSets) == 0 && !m.def.Packages.Update {
		return nil
	}

	err := m.mgr.refresh()
	if err != nil {
		return errors.WithMessage(err, "Failed to refresh")
	}

	if m.def.Packages.Update {
		err = m.mgr.update()
		if err != nil {
			return errors.WithMessage(err, "Failed to update")
		}

		// Run post update hook
		for _, action := range m.def.GetRunnableActions("post-update", imageTarget) {
			err = shared.RunScript(action.Action)
			if err != nil {
				return errors.WithMessage(err, "Failed to run post-update")
			}
		}
	}

	for _, set := range optimizePackageSets(validSets) {
		if set.Action == "install" {
			err = m.mgr.install(set.Packages, set.Flags)
		} else if set.Action == "remove" {
			err = m.mgr.remove(set.Packages, set.Flags)
		}
		if err != nil {
			return errors.WithMessagef(err, "Failed to %s packages", set.Action)
		}
	}

	if m.def.Packages.Cleanup {
		err = m.mgr.clean()
		if err != nil {
			return errors.WithMessage(err, "Failed to clean up packages")
		}
	}

	return nil
}

// ManageRepositories manages repositories.
func (m *Manager) ManageRepositories(imageTarget shared.ImageTarget) error {
	var err error

	if m.def.Packages.Repositories == nil || len(m.def.Packages.Repositories) == 0 {
		return nil
	}

	for _, repo := range m.def.Packages.Repositories {
		if !shared.ApplyFilter(&repo, m.def.Image.Release, m.def.Image.ArchitectureMapped, m.def.Image.Variant, m.def.Targets.Type, imageTarget) {
			continue
		}

		// Run template on repo.URL
		repo.URL, err = shared.RenderTemplate(repo.URL, m.def)
		if err != nil {
			return errors.WithMessage(err, "Failed to render template")
		}

		// Run template on repo.Key
		repo.Key, err = shared.RenderTemplate(repo.Key, m.def)
		if err != nil {
			return errors.WithMessage(err, "Failed to render template")
		}

		err = m.mgr.manageRepository(repo)
		if err != nil {
			return errors.WithMessagef(err, "Error for repository %s", repo.Name)
		}
	}

	return nil
}

// optimizePackageSets groups consecutive package sets with the same action to
// reduce the amount of calls to manager.{Install,Remove}(). It still honors the
// order of execution.
func optimizePackageSets(sets []shared.DefinitionPackagesSet) []shared.DefinitionPackagesSet {
	if len(sets) < 2 {
		return sets
	}

	var newSets []shared.DefinitionPackagesSet

	action := sets[0].Action
	packages := sets[0].Packages
	flags := sets[0].Flags

	for i := 1; i < len(sets); i++ {
		if sets[i].Action == sets[i-1].Action && strings.Join(sets[i].Flags, " ") == strings.Join(sets[i-1].Flags, " ") {
			packages = append(packages, sets[i].Packages...)
		} else {
			newSets = append(newSets, shared.DefinitionPackagesSet{
				Action:   action,
				Packages: packages,
				Flags:    flags,
			})

			action = sets[i].Action
			packages = sets[i].Packages
			flags = sets[i].Flags
		}
	}

	newSets = append(newSets, shared.DefinitionPackagesSet{
		Action:   action,
		Packages: packages,
		Flags:    flags,
	})

	return newSets
}
