package managers

import (
	"context"

	"github.com/lxc/distrobuilder/shared"
	"github.com/sirupsen/logrus"
)

type common struct {
	commands   managerCommands
	flags      managerFlags
	hooks      managerHooks
	logger     *logrus.Logger
	definition shared.Definition
	ctx        context.Context
}

func (c *common) init(ctx context.Context, logger *logrus.Logger, definition shared.Definition) {
	c.logger = logger
	c.definition = definition
	c.ctx = ctx
}

// Install installs packages to the rootfs.
func (c *common) install(pkgs, flags []string) error {
	if len(c.flags.install) == 0 || pkgs == nil || len(pkgs) == 0 {
		return nil
	}

	args := append(c.flags.global, c.flags.install...)
	args = append(args, flags...)
	args = append(args, pkgs...)

	return shared.RunCommand(c.ctx, nil, nil, c.commands.install, args...)
}

// Remove removes packages from the rootfs.
func (c *common) remove(pkgs, flags []string) error {
	if len(c.flags.remove) == 0 || pkgs == nil || len(pkgs) == 0 {
		return nil
	}

	args := append(c.flags.global, c.flags.remove...)
	args = append(args, flags...)
	args = append(args, pkgs...)

	return shared.RunCommand(c.ctx, nil, nil, c.commands.remove, args...)
}

// Clean cleans up cached files used by the package managers.
func (c *common) clean() error {
	var err error

	if len(c.flags.clean) == 0 {
		return nil
	}

	args := append(c.flags.global, c.flags.clean...)

	err = shared.RunCommand(c.ctx, nil, nil, c.commands.clean, args...)
	if err != nil {
		return err
	}

	if c.hooks.clean != nil {
		err = c.hooks.clean()
	}

	return err
}

// Refresh refreshes the local package database.
func (c *common) refresh() error {
	if len(c.flags.refresh) == 0 {
		return nil
	}

	if c.hooks.preRefresh != nil {
		err := c.hooks.preRefresh()
		if err != nil {
			return err
		}
	}

	args := append(c.flags.global, c.flags.refresh...)

	return shared.RunCommand(c.ctx, nil, nil, c.commands.refresh, args...)
}

// Update updates all packages.
func (c *common) update() error {
	if len(c.flags.update) == 0 {
		return nil
	}

	args := append(c.flags.global, c.flags.update...)

	return shared.RunCommand(c.ctx, nil, nil, c.commands.update, args...)
}

// SetInstallFlags overrides the default install flags.
func (c *common) setInstallFlags(flags ...string) {
	c.flags.install = flags
}

func (c *common) manageRepository(repo shared.DefinitionPackagesRepository) error {
	return nil
}
