// fw is the CLI tool for the FW framework.
//
// Usage:
//
//	fw init [project-name]        Create a new FW project
//	fw build [flags]              Run pre-build, build and post-build flow
//
// Run "fw <command> -h" for details on each command.
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		if err := runInit(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "fw init: %v\n", err)
			os.Exit(1)
		}
	case "build":
		if err := runBuild(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "fw build: %v\n", err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		fmt.Printf("fw %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "fw: unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`fw - FW Framework CLI

Usage:
  fw <command> [arguments]

Commands:
  init [project-name]   Create a new FW project with standard directory layout
  build [flags]         Run pre-build, build and post-build flow
  version               Print version

Run "fw <command> -h" for more information on a command.
`)
}
