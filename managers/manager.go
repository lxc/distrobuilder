package managers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lxc/distrobuilder/shared"
)

// ErrUnknownManager represents the unknown manager error.
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
	mgr    manager
	def    shared.Definition
	ctx    context.Context
	logger *logrus.Logger
}

type manager interface {
	init(ctx context.Context, logger *logrus.Logger, definition shared.Definition)
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
	"anise":      func() manager { return &anise{} },
	"opkg":       func() manager { return &opkg{} },
	"pacman":     func() manager { return &pacman{} },
	"portage":    func() manager { return &portage{} },
	"slackpkg":   func() manager { return &slackpkg{} },
	"xbps":       func() manager { return &xbps{} },
	"yum":        func() manager { return &yum{} },
	"zypper":     func() manager { return &zypper{} },
}

// Load loads and initializes a downloader.
func Load(ctx context.Context, managerName string, logger *logrus.Logger, definition shared.Definition) (*Manager, error) {
	df, ok := managers[managerName]
	if !ok {
		return nil, ErrUnknownManager
	}

	d := df()

	d.init(ctx, logger, definition)

	err := d.load()
	if err != nil {
		return nil, fmt.Errorf("Failed to load manager %q: %w", managerName, err)
	}

	return &Manager{def: definition, mgr: d, ctx: ctx, logger: logger}, nil
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
		return fmt.Errorf("Failed to refresh: %w", err)
	}

	if m.def.Packages.Update {
		err = m.mgr.update()
		if err != nil {
			return fmt.Errorf("Failed to update: %w", err)
		}

		m.logger.WithField("trigger", "post-update").Info("Running hooks")

		// Run post update hook
		for _, action := range m.def.GetRunnableActions("post-update", imageTarget) {
			if action.Pongo {
				action.Action, err = shared.RenderTemplate(action.Action, m.def)
				if err != nil {
					return fmt.Errorf("Failed to render action: %w", err)
				}
			}

			err = shared.RunScript(m.ctx, action.Action)
			if err != nil {
				return fmt.Errorf("Failed to run post-update: %w", err)
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
			return fmt.Errorf("Failed to %s packages: %w", set.Action, err)
		}
	}

	if m.def.Packages.Cleanup {
		err = m.mgr.clean()
		if err != nil {
			return fmt.Errorf("Failed to clean up packages: %w", err)
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
			return fmt.Errorf("Failed to render template: %w", err)
		}

		// Run template on repo.Key
		repo.Key, err = shared.RenderTemplate(repo.Key, m.def)
		if err != nil {
			return fmt.Errorf("Failed to render template: %w", err)
		}

		err = m.mgr.manageRepository(repo)
		if err != nil {
			return fmt.Errorf("Error for repository %s: %w", repo.Name, err)
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
