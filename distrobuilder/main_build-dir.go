package main

import "github.com/spf13/cobra"

type cmdBuildDir struct {
	cmd    *cobra.Command
	global *cmdGlobal
}

func (c *cmdBuildDir) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-dir <filename|-> <target dir>",
		Short: "Build plain rootfs",
		Args:  cobra.ExactArgs(2),
		RunE:  c.global.preRunBuild,
	}

	c.cmd = cmd
	return cmd
}
