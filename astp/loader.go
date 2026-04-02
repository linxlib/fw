package astp

import (
	"encoding/json"
	"fmt"
	"io"
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

// LoadFromBytes parses a Project from in-memory JSON data.
// This is typically used with go:embed to load pre-generated .astp.json
// without requiring the file to exist on disk.
func LoadFromBytes(data []byte) (*Project, error) {
	p := &Project{}
	if err := json.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("unmarshal astp data: %w", err)
	}
	return p, nil
}

// LoadFromReader parses a Project from an io.Reader.
func LoadFromReader(r io.Reader) (*Project, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read astp data: %w", err)
	}
	return LoadFromBytes(data)
}
