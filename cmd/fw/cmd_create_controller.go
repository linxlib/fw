package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type createControllerData struct {
	Name           string
	NameLower      string
	TypeName       string
	MethodName     string
	Route          string
	GenerateMethod bool
}

func newCreateControllerCmd() *cobra.Command {
	var name string
	var route string

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Create a controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			interactive := strings.TrimSpace(name) == ""
			if interactive {
				reader := bufio.NewReader(os.Stdin)
				var err error
				name, err = promptRequiredInput(reader, "Controller name")
				if err != nil {
					return fmt.Errorf("fw create controller: read name: %w", err)
				}
				route, err = promptInputWithDefault(reader, "Controller route", "/api")
				if err != nil {
					return fmt.Errorf("fw create controller: read route: %w", err)
				}
			}

			base, typeName, err := normalizeName(name, "Controller")
			if err != nil {
				return fmt.Errorf("fw create controller: %w", err)
			}

			r := strings.TrimSpace(route)
			if r == "" {
				r = "/api"
			}

			data := createControllerData{
				Name:           base,
				NameLower:      strings.ToLower(base),
				TypeName:       typeName,
				MethodName:     "Get" + base,
				Route:          r,
				GenerateMethod: !interactive,
			}

			content, err := renderTemplate("templates/create/controller.go.tmpl", data)
			if err != nil {
				return fmt.Errorf("fw create controller: render template: %w", err)
			}

			dir := filepath.Join(".", "controllers")
			if err := ensureDir(dir); err != nil {
				return fmt.Errorf("fw create controller: create directory: %w", err)
			}

			filePath := filepath.Join(dir, toSnakeCase(typeName)+".go")
			if err := writeNewFile(filePath, content); err != nil {
				return fmt.Errorf("fw create controller: %w", err)
			}

			fmt.Printf("Created %s\n", filePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "controller name, for example User")
	cmd.Flags().StringVar(&route, "route", "/api", "controller base route")

	return cmd
}
