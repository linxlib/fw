package main

import (
	"github.com/linxlib/fw/cmd/commands"
	"github.com/urfave/cli/v2"
	"os"
)

func main() {
	app := &cli.App{
		Name:        "fw",
		Version:     "v1.0.0@beta",
		Description: "helper for github.com/linxlib/fw",
		Action: func(*cli.Context) error {
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:    "init",
				Aliases: []string{"i"},
				Usage:   "init fw project",
				Action:  commands.Init,
			},
			{
				Name:    "gen",
				Aliases: []string{"g"},
				Usage:   "generate project metadata to gen.json",
				Action:  commands.Generate,
			},
			{
				Name:        "config",
				Aliases:     []string{"c"},
				Usage:       "config fw project",
				Description: "",
				UsageText: `fw config <key> <value> -> write config to config/config.yaml
fw config <key> -> read config from config/config.yaml`,
				Action: commands.Config,
			},
			{
				Name:    "build",
				Aliases: []string{"b"},
				Usage:   "build project",
				UsageText: `fw build linux amd64 -> build project for linux amd64
fw build windows -> build project for windows amd64(default)`,
				Action: commands.Build,
			},
			{
				Name:      "add",
				Aliases:   []string{"a"},
				Usage:     "add middleware/mapper/controller",
				UsageText: `fw add middleware/mapper/controller <name> -> add middleware/mapper/controller to project`,
				Action:    commands.Add,
			},
		},
	}
	app.Run(os.Args)
}
