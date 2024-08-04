package yaml

import (
	"gopkg.in/yaml.v3"
)

var (
	// Marshal is exported by gin/json package.
	Marshal = yaml.Marshal
	// Unmarshal is exported by gin/json package.
	Unmarshal = yaml.Unmarshal
	// MarshalIndent is exported by gin/json package.
	MarshalIndent = yaml.Marshal
	// NewDecoder is exported by gin/json package.
	NewDecoder = yaml.NewDecoder
	// NewEncoder is exported by gin/json package.
	NewEncoder = yaml.NewEncoder
)
