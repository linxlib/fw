package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type createServiceData struct {
	Name     string
	TypeName string
	CtorName string
}

func newCreateServiceCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "service",
		Short: "Create a service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(name) == "" {
				reader := bufio.NewReader(os.Stdin)
				var err error
				name, err = promptRequiredInput(reader, "Service name")
				if err != nil {
					return fmt.Errorf("fw create service: read name: %w", err)
				}
			}

			base, typeName, err := normalizeName(name, "Service")
			if err != nil {
				return fmt.Errorf("fw create service: %w", err)
			}

			data := createServiceData{
				Name:     base,
				TypeName: typeName,
				CtorName: "New" + typeName,
			}

			content, err := renderTemplate("templates/create/service.go.tmpl", data)
			if err != nil {
				return fmt.Errorf("fw create service: render template: %w", err)
			}

			dir := filepath.Join(".", "services")
			if err := ensureDir(dir); err != nil {
				return fmt.Errorf("fw create service: create directory: %w", err)
			}

			filePath := filepath.Join(dir, toSnakeCase(typeName)+".go")
			if err := writeNewFile(filePath, content); err != nil {
				return fmt.Errorf("fw create service: %w", err)
			}

			fmt.Printf("Created %s\n", filePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "service name, for example User")

	return cmd
}
