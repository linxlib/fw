package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type initTemplateData struct {
	Name   string
	Module string
}

type scaffoldTemplate struct {
	target string
	source string
}

var scaffoldTemplates = []scaffoldTemplate{
	{target: "main.go", source: "templates/main.go.tmpl"},
	{target: "controllers/hello_world_controller.go", source: "templates/controllers/hello_world_controller.go.tmpl"},
	{target: "services/hello_service.go", source: "templates/services/hello_service.go.tmpl"},
	{target: "middlewares/log.go", source: "templates/middlewares/log.go.tmpl"},
	{target: "models/hello.go", source: "templates/models/hello.go.tmpl"},
	{target: "config/app.yaml", source: "templates/config/app.yaml.tmpl"},
	{target: ".gitignore", source: "templates/gitignore.tmpl"},
}

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [project-name]",
		Short: "Create a new FW project with standard directory layout",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := resolveInitOptions(args)
			if err != nil {
				return fmt.Errorf("fw init: %w", err)
			}

			if err := runInit(opts); err != nil {
				return fmt.Errorf("fw init: %w", err)
			}
			return nil
		},
	}

	return cmd
}

type initOptions struct {
	projectName string
	moduleName  string
	root        string
	createdDir  bool
}

func resolveInitOptions(args []string) (initOptions, error) {
	if len(args) == 1 {
		name := strings.TrimSpace(args[0])
		if name == "" {
			return initOptions{}, fmt.Errorf("project name cannot be empty")
		}

		root, err := filepath.Abs(name)
		if err != nil {
			return initOptions{}, err
		}

		return initOptions{
			projectName: name,
			moduleName:  name,
			root:        root,
			createdDir:  true,
		}, nil
	}

	root, err := filepath.Abs(".")
	if err != nil {
		return initOptions{}, err
	}

	name := filepath.Base(root)
	if name == "." || name == string(filepath.Separator) {
		return initOptions{}, fmt.Errorf("cannot derive project name from current directory")
	}

	return initOptions{
		projectName: name,
		moduleName:  name,
		root:        root,
		createdDir:  false,
	}, nil
}

func runInit(opts initOptions) error {
	root := opts.root
	if opts.createdDir {
		if _, err := os.Stat(root); err == nil {
			return fmt.Errorf("directory %q already exists", root)
		} else if !os.IsNotExist(err) {
			return err
		}

		if err := os.MkdirAll(root, 0755); err != nil {
			return fmt.Errorf("create project directory: %w", err)
		}
	} else {
		if err := ensureDirectoryBasicallyEmpty(root); err != nil {
			return err
		}
	}

	if opts.createdDir {
		fmt.Printf("Creating project %s ...\n", opts.projectName)
	} else {
		fmt.Printf("Initializing project in current directory ...\n")
	}

	data := initTemplateData{Name: opts.projectName, Module: opts.moduleName}

	// Create directory structure
	dirs := []string{
		"controllers",
		"services",
		"middlewares",
		"models",
		"config",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	for _, item := range scaffoldTemplates {
		content, err := renderTemplate(item.source, data)
		if err != nil {
			return fmt.Errorf("render %s: %w", item.target, err)
		}

		p := filepath.Join(root, item.target)
		if err := writeNewFile(p, content); err != nil {
			return fmt.Errorf("write %s: %w", item.target, err)
		}
	}

	// Run go mod init
	fmt.Printf("Initializing Go module ...\n")
	cmd := exec.Command("go", "mod", "init", opts.moduleName)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod init: %w", err)
	}

	// Run go mod tidy to fetch dependencies
	fmt.Printf("Fetching dependencies ...\n")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}

	fmt.Printf("\nProject created at %s\n\n", root)
	fmt.Printf("Next steps:\n")
	if opts.createdDir {
		fmt.Printf("  cd %s\n", opts.projectName)
	}
	fmt.Printf("  fw build              # run pre-build, build and post-build\n")
	fmt.Printf("  go run .              # run in development mode\n")
	return nil
}

func ensureDirectoryBasicallyEmpty(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	return fmt.Errorf("current directory must be basically empty to run init")
}
