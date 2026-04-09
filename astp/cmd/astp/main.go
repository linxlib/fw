package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/linxlib/fw/v2/astp"
)

func main() {
	dir := flag.String("dir", ".", "directory to parse")
	output := flag.String("o", "", "output file path")
	autoReg := flag.Bool("autoreg", true, "generate fw autoreg file")
	autoRegOutput := flag.String("autoreg-o", "", "autoreg output file path")
	exportedOnly := flag.Bool("exported", false, "only parse exported (public) types, functions, fields")
	flag.Parse()

	targetDir := flag.Arg(0)
	if targetDir != "" {
		*dir = targetDir
	}

	g := astp.NewGenerator()
	g.ExportedOnly = *exportedOnly
	err := g.Generate(*dir, *output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *autoReg {
		ag := astp.NewAutoRegGenerator()
		ag.ExportedOnly = *exportedOnly
		if err := ag.Generate(*dir, *autoRegOutput); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	outFile := *output
	if outFile == "" {
		outFile = *dir + string(os.PathSeparator) + astp.DefaultOutputFile
	}
	fmt.Printf("Generated: %s\n", outFile)
	if *autoReg {
		autoRegOut := *autoRegOutput
		if autoRegOut == "" {
			autoRegOut = *dir + string(os.PathSeparator) + astp.DefaultAutoRegOutputFile
		}
		fmt.Printf("Generated: %s\n", autoRegOut)
	}
}
