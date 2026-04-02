package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ProjectDir string `yaml:"project_dir"`
	Server     ServerConfig
	Log        LogConfig
	OpenAPI    OpenAPIConfig `yaml:"openapi"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LogConfig struct {
	Level      string `yaml:"level"`
	Output     string `yaml:"output"`
	FilePath   string `yaml:"file_path"`
	EnableFile bool   `yaml:"enable_file"`
}

type OpenAPIConfig struct {
	Enabled bool   `yaml:"enabled"`
	Output  string `yaml:"output"`
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

func Default() Config {
	return Config{
		ProjectDir: ".",
		Server:     ServerConfig{Host: "0.0.0.0", Port: 8080},
		Log:        LogConfig{Level: "info", Output: "console", FilePath: "logs/fw.log", EnableFile: false},
		OpenAPI:    OpenAPIConfig{Enabled: true, Output: "openapi.json", Title: "FW API", Version: "1.0.0"},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse yaml: %w", err)
		}
	}
	applyEnv("FW", reflect.ValueOf(&cfg).Elem(), nil)
	return cfg, nil
}

func applyEnv(prefix string, v reflect.Value, path []string) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		tf := t.Field(i)
		if !f.CanSet() {
			continue
		}
		name := tf.Tag.Get("yaml")
		if name == "" {
			name = strings.ToLower(tf.Name)
		}
		name = strings.Split(name, ",")[0]
		if name == "-" {
			continue
		}
		currPath := append(path, name)
		if f.Kind() == reflect.Struct {
			applyEnv(prefix, f, currPath)
			continue
		}
		envKey := prefix + "_" + strings.ToUpper(strings.Join(currPath, "_"))
		value := os.Getenv(envKey)
		if value == "" {
			continue
		}
		setValue(f, value)
	}
}

func setValue(v reflect.Value, value string) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n, err := strconv.ParseInt(value, 10, 64); err == nil {
			v.SetInt(n)
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(value); err == nil {
			v.SetBool(b)
		}
	}
}
