package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "fw",
		Short:        "FW Framework CLI",
		SilenceUsage: true,
	}

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newBuildCmd())
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("fw %s\n", version)
		},
	})

	return cmd
}
