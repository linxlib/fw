package main

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"unicode"
)

//go:embed templates/* templates/controllers/* templates/services/* templates/middlewares/* templates/models/* templates/config/* templates/create/*
var scaffoldFS embed.FS

var scaffoldNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

func renderTemplate(path string, data any) (string, error) {
	raw, err := scaffoldFS.ReadFile(path)
	if err != nil {
		return "", err
	}

	tpl, err := template.New(filepath.Base(path)).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func normalizeName(name string, suffix string) (string, string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", "", fmt.Errorf("name is required")
	}
	if !scaffoldNamePattern.MatchString(trimmed) {
		return "", "", fmt.Errorf("name must start with a letter and contain only letters or digits")
	}

	base := trimmed
	if strings.HasSuffix(base, suffix) {
		base = strings.TrimSuffix(base, suffix)
	}
	if base == "" {
		return "", "", fmt.Errorf("name must include a prefix before %s", suffix)
	}

	return base, base + suffix, nil
}

func toSnakeCase(s string) string {
	if s == "" {
		return s
	}

	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func writeNewFile(path string, content string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file %q already exists", path)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	return nil
}

func promptRequiredInput(r *bufio.Reader, label string) (string, error) {
	for {
		fmt.Printf("%s: ", label)
		text, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}

		value := strings.TrimSpace(text)
		if value != "" {
			return value, nil
		}

		fmt.Println("input is required")
	}
}

func promptInputWithDefault(r *bufio.Reader, label string, defaultValue string) (string, error) {
	fmt.Printf("%s [%s]: ", label, defaultValue)
	text, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}

	value := strings.TrimSpace(text)
	if value == "" {
		return defaultValue, nil
	}

	return value, nil
}
