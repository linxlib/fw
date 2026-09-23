package astp

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go/ast"
)

var annotationRegex = regexp.MustCompile(`@(\w+)(?:\s+(.+))?`)

func parseDoc(doc *ast.CommentGroup) *CommentGroup {
	if doc == nil {
		return nil
	}

	cg := &CommentGroup{}
	for _, c := range doc.List {
		text := strings.TrimPrefix(c.Text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)

		if strings.HasPrefix(text, "@") {
			ann := parseAnnotation(text)
			if ann.Name != "" {
				cg.Annotations = append(cg.Annotations, ann.Name)
				cg.ParsedAnnotations = append(cg.ParsedAnnotations, ann)
			}
		}

		cg.List = append(cg.List, &Comment{Text: text})
	}

	return cg
}

func parseAnnotation(line string) *Annotation {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "@") {
		return &Annotation{}
	}

	body := strings.TrimPrefix(line, "@")
	name := body
	rest := ""

	if i := strings.IndexAny(body, " (\t\n\r"); i >= 0 {
		name = strings.TrimSpace(body[:i])
		rest = strings.TrimSpace(body[i:])
	}

	ann := &Annotation{Name: name, Raw: line}
	if ann.Name == "" {
		return ann
	}

	if strings.HasPrefix(rest, "(") {
		// closeIdx 而非 close: close 是内建函数名, 用作变量名会遮蔽它.
		closeIdx := strings.LastIndex(rest, ")")
		if closeIdx > 0 {
			inside := strings.TrimSpace(rest[1:closeIdx])
			parseAnnotationParams(inside, ann)
			return ann
		}
	}

	if rest != "" {
		parseAnnotationParams(rest, ann)
	}

	return ann
}

func parseAnnotationParams(input string, ann *Annotation) {
	parts := splitByComma(input)
	if len(parts) == 0 {
		parts = strings.Fields(input)
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if idx := strings.Index(part, "="); idx > 0 {
			if ann.KV == nil {
				ann.KV = make(map[string]string)
			}
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			value = trimQuotes(value)
			if key != "" {
				ann.KV[key] = value
			}
			continue
		}
		ann.Args = append(ann.Args, trimQuotes(part))
	}
}

func splitByComma(input string) []string {
	var out []string
	if strings.TrimSpace(input) == "" {
		return out
	}
	var b strings.Builder
	inQuote := byte(0)
	escaped := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if escaped {
			b.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			b.WriteByte(ch)
			continue
		}
		if inQuote != 0 {
			if ch == inQuote {
				inQuote = 0
			}
			b.WriteByte(ch)
			continue
		}
		if ch == '\'' || ch == '"' {
			inQuote = ch
			b.WriteByte(ch)
			continue
		}
		if ch == ',' {
			out = append(out, strings.TrimSpace(b.String()))
			b.Reset()
			continue
		}
		b.WriteByte(ch)
	}
	if b.Len() > 0 {
		out = append(out, strings.TrimSpace(b.String()))
	}
	return out
}

func trimQuotes(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	unq, err := strconv.Unquote(v)
	if err == nil {
		return unq
	}
	v = strings.TrimPrefix(v, `"`)
	v = strings.TrimSuffix(v, `"`)
	v = strings.TrimPrefix(v, `'`)
	v = strings.TrimSuffix(v, `'`)
	return v
}

func ParseAnnotations(doc *CommentGroup) []string {
	if doc == nil {
		return nil
	}
	return doc.Annotations
}

func HasAnnotation(doc *CommentGroup, name string) bool {
	if doc == nil {
		return false
	}
	return slices.Contains(doc.Annotations, name)
}

func GetAnnotationValue(doc *CommentGroup, name string) string {
	if a := GetAnnotation(doc, name); a != nil {
		if len(a.Args) > 0 {
			return a.Args[0]
		}
		for _, c := range doc.List {
			matches := annotationRegex.FindStringSubmatch(c.Text)
			if len(matches) >= 3 && matches[1] == name {
				return strings.TrimSpace(matches[2])
			}
		}
	}
	if doc == nil {
		return ""
	}
	for _, c := range doc.List {
		matches := annotationRegex.FindStringSubmatch(c.Text)
		if len(matches) >= 3 && matches[1] == name {
			return strings.TrimSpace(matches[2])
		}
	}
	return ""
}

func GetAnnotation(doc *CommentGroup, name string) *Annotation {
	if doc == nil {
		return nil
	}
	for _, ann := range doc.ParsedAnnotations {
		if ann != nil && ann.Name == name {
			return ann
		}
	}
	return nil
}

func GetAnnotations(doc *CommentGroup, name string) []*Annotation {
	if doc == nil {
		return nil
	}
	var out []*Annotation
	for _, ann := range doc.ParsedAnnotations {
		if ann != nil && ann.Name == name {
			out = append(out, ann)
		}
	}
	return out
}
