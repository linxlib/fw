package astp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const DefaultOutputFile = ".astp.json"

type Generator struct {
	parser       *Parser
	ExportedOnly bool
}

func NewGenerator() *Generator {
	return &Generator{
		parser: NewParser(),
	}
}

func (g *Generator) Generate(dir string, outputFile string) error {
	g.parser.ExportedOnly = g.ExportedOnly
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("get absolute path: %w", err)
	}

	project, err := g.parser.ParseProject(absDir)
	if err != nil {
		return fmt.Errorf("parse project: %w", err)
	}

	if outputFile == "" {
		outputFile = filepath.Join(absDir, DefaultOutputFile)
	} else {
		if !filepath.IsAbs(outputFile) {
			outputFile = filepath.Join(absDir, outputFile)
		}
	}

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project: %w", err)
	}

	err = os.WriteFile(outputFile, data, 0644)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func Generate(dir string) error {
	g := NewGenerator()
	return g.Generate(dir, "")
}

func GenerateToFile(dir, outputFile string) error {
	g := NewGenerator()
	return g.Generate(dir, outputFile)
}
