package main

import "github.com/spf13/cobra"

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create framework components",
	}

	cmd.AddCommand(newCreateControllerCmd())
	cmd.AddCommand(newCreateMiddlewareCmd())
	cmd.AddCommand(newCreateServiceCmd())

	return cmd
}
