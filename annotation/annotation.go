package annotation

import (
	"strings"

	"github.com/linxlib/fw/v2/astp"
)

type Parsed struct {
	Name string
	Args []string
	KV   map[string]string
	Raw  string
}

func FromDoc(doc *astp.CommentGroup) []Parsed {
	if doc == nil {
		return nil
	}
	if len(doc.ParsedAnnotations) > 0 {
		out := make([]Parsed, 0, len(doc.ParsedAnnotations))
		for _, ann := range doc.ParsedAnnotations {
			if ann == nil || ann.Name == "" {
				continue
			}
			out = append(out, Parsed{Name: ann.Name, Args: ann.Args, KV: ann.KV, Raw: ann.Raw})
		}
		return out
	}
	var out []Parsed
	for _, name := range doc.Annotations {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out = append(out, Parsed{Name: name})
	}
	return out
}

func LastByName(items []Parsed) map[string]Parsed {
	out := make(map[string]Parsed)
	for _, item := range items {
		if item.Name == "" {
			continue
		}
		out[item.Name] = item
	}
	return out
}
