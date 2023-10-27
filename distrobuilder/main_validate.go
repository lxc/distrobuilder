package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type cmdValidate struct {
	cmdValidate *cobra.Command
	global      *cmdGlobal
}

func (c *cmdValidate) command() *cobra.Command {
	c.cmdValidate = &cobra.Command{
		Use:   "validate <filename|->",
		Short: "Validate definition file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the image definition
			_, err := getDefinition(args[0], c.global.flagOptions)
			if err != nil {
				return fmt.Errorf("Failed to get definition: %w", err)
			}

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	c.cmdValidate.Flags().StringSliceVarP(&c.global.flagOptions, "options", "o",
		[]string{}, "Override options (list of key=value)"+"``")

	return c.cmdValidate
}
