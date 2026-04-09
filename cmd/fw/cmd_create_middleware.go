package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type createMiddlewareData struct {
	Name     string
	TypeName string
}

func newCreateMiddlewareCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "middleware",
		Short: "Create a middleware",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(name) == "" {
				reader := bufio.NewReader(os.Stdin)
				var err error
				name, err = promptRequiredInput(reader, "Middleware name")
				if err != nil {
					return fmt.Errorf("fw create middleware: read name: %w", err)
				}
			}

			base, typeName, err := normalizeName(name, "Middleware")
			if err != nil {
				return fmt.Errorf("fw create middleware: %w", err)
			}

			data := createMiddlewareData{
				Name:     base,
				TypeName: typeName,
			}

			content, err := renderTemplate("templates/create/middleware.go.tmpl", data)
			if err != nil {
				return fmt.Errorf("fw create middleware: render template: %w", err)
			}

			dir := filepath.Join(".", "middlewares")
			if err := ensureDir(dir); err != nil {
				return fmt.Errorf("fw create middleware: create directory: %w", err)
			}

			filePath := filepath.Join(dir, toSnakeCase(base)+".go")
			if err := writeNewFile(filePath, content); err != nil {
				return fmt.Errorf("fw create middleware: %w", err)
			}

			fmt.Printf("Created %s\n", filePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "middleware name, for example Authorization")

	return cmd
}
