package astp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Loader struct {
	project *Project
}

func NewLoader() *Loader {
	return &Loader{}
}

func (l *Loader) LoadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	l.project = &Project{}
	err = json.Unmarshal(data, l.project)
	if err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	return nil
}

func (l *Loader) Load(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("get absolute path: %w", err)
	}

	return l.LoadFile(filepath.Join(absDir, DefaultOutputFile))
}

func (l *Loader) Project() *Project {
	return l.project
}

func Load(path string) (*Project, error) {
	l := NewLoader()
	var err error
	if filepath.Ext(path) == ".json" {
		err = l.LoadFile(path)
	} else {
		err = l.Load(path)
	}
	if err != nil {
		return nil, err
	}
	return l.Project(), nil
}
