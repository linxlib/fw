package config

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ProjectDir  string `yaml:"project_dir"`
	Server      ServerConfig
	Log         LogConfig
	Recovery    RecoveryConfig     `yaml:"recovery"`
	OpenAPI     OpenAPIConfig      `yaml:"openapi"`
	Middlewares map[string]Section `yaml:"middlewares"`
}

type Section struct {
	data map[string]any
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LogConfig struct {
	Level          string `yaml:"level"`
	Output         string `yaml:"output"`
	FilePath       string `yaml:"file_path"`
	EnableFile     bool   `yaml:"enable_file"`
	RequestEnabled bool   `yaml:"request_enabled"`
}

type RecoveryConfig struct {
	Enabled           bool `yaml:"enabled"`
	ReturnStackToBody bool `yaml:"return_stack_to_body"`
}

type OpenAPIConfig struct {
	Enabled bool   `yaml:"enabled"`
	Output  string `yaml:"output"`
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

func Default() Config {
	return Config{
		ProjectDir:  ".",
		Server:      ServerConfig{Host: "0.0.0.0", Port: 8080},
		Log:         LogConfig{Level: "info", Output: "console", FilePath: "logs/fw.log", EnableFile: false, RequestEnabled: true},
		Recovery:    RecoveryConfig{Enabled: true, ReturnStackToBody: true},
		OpenAPI:     OpenAPIConfig{Enabled: true, Output: "openapi.json", Title: "FW API", Version: "1.0.0"},
		Middlewares: map[string]Section{},
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
	applyDynamicMiddlewareEnv("FW", &cfg)
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

func NewSection(data map[string]any) Section {
	if data == nil {
		data = map[string]any{}
	}
	return Section{data: normalizeMap(data)}
}

func (s Section) Has(key string) bool {
	_, ok := s.lookup(key)
	return ok
}

func (s Section) Raw(key string) any {
	v, _ := s.lookup(key)
	return v
}

func (s Section) Get(key string) string {
	return s.GetString(key)
}

func (s Section) GetString(key string) string {
	return s.GetStringDefault(key, "")
}

func (s Section) GetStringDefault(key string, defaultValue string) string {
	v, ok := s.lookup(key)
	if !ok {
		return defaultValue
	}
	return toString(v)
}

func (s Section) GetInt(key string) int {
	return s.GetIntDefault(key, 0)
}

func (s Section) GetIntDefault(key string, defaultValue int) int {
	v, ok := s.lookup(key)
	if !ok {
		return defaultValue
	}
	return toInt(v)
}

func (s Section) GetInt64(key string) int64 {
	return s.GetInt64Default(key, 0)
}

func (s Section) GetInt64Default(key string, defaultValue int64) int64 {
	v, ok := s.lookup(key)
	if !ok {
		return defaultValue
	}
	return toInt64(v)
}

func (s Section) GetBool(key string) bool {
	return s.GetBoolDefault(key, false)
}

func (s Section) GetBoolDefault(key string, defaultValue bool) bool {
	v, ok := s.lookup(key)
	if !ok {
		return defaultValue
	}
	return toBool(v)
}

func (s Section) GetFloat(key string) float64 {
	return s.GetFloatDefault(key, 0)
}

func (s Section) GetFloatDefault(key string, defaultValue float64) float64 {
	v, ok := s.lookup(key)
	if !ok {
		return defaultValue
	}
	return toFloat64(v)
}

func (s Section) GetFloat64(key string) float64 {
	return s.GetFloat(key)
}

func (s Section) GetFloat64Default(key string, defaultValue float64) float64 {
	return s.GetFloatDefault(key, defaultValue)
}

func (s Section) GetDuration(key string) time.Duration {
	return s.GetDurationDefault(key, 0)
}

func (s Section) GetDurationDefault(key string, defaultValue time.Duration) time.Duration {
	v, ok := s.lookup(key)
	if !ok {
		return defaultValue
	}
	return toDuration(v, defaultValue)
}

func (s Section) GetStrings(key string) []string {
	return s.GetStringsDefault(key, nil)
}

func (s Section) GetStringsDefault(key string, defaultValue []string) []string {
	v, ok := s.lookup(key)
	if !ok {
		return cloneStrings(defaultValue)
	}
	items, ok := toStrings(v)
	if !ok {
		return cloneStrings(defaultValue)
	}
	return items
}

func (s Section) MustGet(key string) string {
	v, ok := s.lookup(key)
	if !ok {
		panic(fmt.Sprintf("config section key %q not found", key))
	}
	return toString(v)
}

func (s Section) Sub(key string) *Section {
	v, ok := s.lookup(key)
	if !ok {
		empty := NewSection(nil)
		return &empty
	}

	switch child := v.(type) {
	case Section:
		cpy := child
		return &cpy
	case *Section:
		if child != nil {
			return child
		}
	case map[string]any:
		next := NewSection(child)
		return &next
	case map[any]any:
		next := NewSection(convertMapAny(child))
		return &next
	}

	empty := NewSection(nil)
	return &empty
}

func (s *Section) UnmarshalYAML(node *yaml.Node) error {
	if node == nil {
		s.data = map[string]any{}
		return nil
	}
	var raw map[string]any
	if err := node.Decode(&raw); err != nil {
		return err
	}
	s.data = normalizeMap(raw)
	return nil
}

func (s Section) lookup(key string) (any, bool) {
	if s.data == nil {
		return nil, false
	}
	v, ok := s.data[key]
	return v, ok
}

func normalizeMap(data map[string]any) map[string]any {
	out := make(map[string]any, len(data))
	for k, v := range data {
		out[k] = normalizeValue(v)
	}
	return out
}

func normalizeValue(v any) any {
	switch value := v.(type) {
	case map[string]any:
		return normalizeMap(value)
	case map[any]any:
		return normalizeMap(convertMapAny(value))
	case []any:
		items := make([]any, len(value))
		for i := range value {
			items[i] = normalizeValue(value[i])
		}
		return items
	default:
		return v
	}
}

func convertMapAny(data map[any]any) map[string]any {
	out := make(map[string]any, len(data))
	for k, v := range data {
		out[fmt.Sprint(k)] = normalizeValue(v)
	}
	return out
}

func toString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case bool:
		return strconv.FormatBool(value)
	case int:
		return strconv.Itoa(value)
	case int8:
		return strconv.FormatInt(int64(value), 10)
	case int16:
		return strconv.FormatInt(int64(value), 10)
	case int32:
		return strconv.FormatInt(int64(value), 10)
	case int64:
		return strconv.FormatInt(value, 10)
	case uint:
		return strconv.FormatUint(uint64(value), 10)
	case uint8:
		return strconv.FormatUint(uint64(value), 10)
	case uint16:
		return strconv.FormatUint(uint64(value), 10)
	case uint32:
		return strconv.FormatUint(uint64(value), 10)
	case uint64:
		return strconv.FormatUint(value, 10)
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

func toInt(v any) int {
	return int(toInt64(v))
}

func toInt64(v any) int64 {
	switch value := v.(type) {
	case int:
		return int64(value)
	case int8:
		return int64(value)
	case int16:
		return int64(value)
	case int32:
		return int64(value)
	case int64:
		return value
	case uint:
		return int64(value)
	case uint8:
		return int64(value)
	case uint16:
		return int64(value)
	case uint32:
		return int64(value)
	case uint64:
		return int64(value)
	case float32:
		return int64(value)
	case float64:
		return int64(value)
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err == nil {
			return n
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil {
			return int64(f)
		}
	}
	return 0
}

func toBool(v any) bool {
	switch value := v.(type) {
	case bool:
		return value
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(value))
		if err == nil {
			return b
		}
	case int:
		return value != 0
	case int8:
		return value != 0
	case int16:
		return value != 0
	case int32:
		return value != 0
	case int64:
		return value != 0
	case uint:
		return value != 0
	case uint8:
		return value != 0
	case uint16:
		return value != 0
	case uint32:
		return value != 0
	case uint64:
		return value != 0
	case float32:
		return value != 0
	case float64:
		return value != 0
	}
	return false
}

func toFloat64(v any) float64 {
	switch value := v.(type) {
	case float32:
		return float64(value)
	case float64:
		return value
	case int:
		return float64(value)
	case int8:
		return float64(value)
	case int16:
		return float64(value)
	case int32:
		return float64(value)
	case int64:
		return float64(value)
	case uint:
		return float64(value)
	case uint8:
		return float64(value)
	case uint16:
		return float64(value)
	case uint32:
		return float64(value)
	case uint64:
		return float64(value)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil {
			return f
		}
	}
	return 0
}

func toDuration(v any, defaultValue time.Duration) time.Duration {
	switch value := v.(type) {
	case time.Duration:
		return value
	case string:
		d, err := time.ParseDuration(strings.TrimSpace(value))
		if err == nil {
			return d
		}
	case int:
		return time.Duration(value)
	case int8:
		return time.Duration(value)
	case int16:
		return time.Duration(value)
	case int32:
		return time.Duration(value)
	case int64:
		return time.Duration(value)
	case uint:
		return time.Duration(value)
	case uint8:
		return time.Duration(value)
	case uint16:
		return time.Duration(value)
	case uint32:
		return time.Duration(value)
	case uint64:
		return time.Duration(value)
	}
	return defaultValue
}

func toStrings(v any) ([]string, bool) {
	switch value := v.(type) {
	case []string:
		return cloneStrings(value), true
	case []any:
		items := make([]string, 0, len(value))
		for _, item := range value {
			items = append(items, toString(item))
		}
		return items, true
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return []string{}, true
		}
		parts := strings.Split(trimmed, ",")
		items := make([]string, 0, len(parts))
		for _, part := range parts {
			items = append(items, strings.TrimSpace(part))
		}
		return items, true
	default:
		return nil, false
	}
}

func cloneStrings(items []string) []string {
	if items == nil {
		return nil
	}
	out := make([]string, len(items))
	copy(out, items)
	return out
}

func applyDynamicMiddlewareEnv(prefix string, cfg *Config) {
	if cfg == nil {
		return
	}
	if cfg.Middlewares == nil {
		cfg.Middlewares = map[string]Section{}
	}
	base := prefix + "_MIDDLEWARES_"
	keys := make([]string, 0)
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if !strings.HasPrefix(parts[0], base) {
			continue
		}
		keys = append(keys, parts[0])
		values[parts[0]] = parts[1]
	}
	sort.Strings(keys)
	for _, envKey := range keys {
		applyDynamicMiddlewareEnvEntry(cfg, strings.TrimPrefix(envKey, base), values[envKey])
	}
}

func applyDynamicMiddlewareEnvEntry(cfg *Config, rawKey string, value string) {
	parts := strings.Split(rawKey, "__")
	if len(parts) == 0 {
		return
	}
	head := strings.Trim(parts[0], "_")
	if head == "" {
		return
	}
	segments := strings.Split(head, "_")
	if len(segments) < 2 {
		return
	}
	sectionName := strings.ToLower(strings.TrimSpace(segments[0]))
	path := make([]string, 0, len(segments)-1+len(parts)-1)
	path = append(path, envPartToKey(strings.Join(segments[1:], "_")))
	for _, part := range parts[1:] {
		part = strings.Trim(part, "_")
		if part == "" {
			continue
		}
		path = append(path, envPartToKey(part))
	}
	if len(path) == 0 {
		return
	}
	section := cfg.Middlewares[sectionName]
	section.setPath(path, value)
	cfg.Middlewares[sectionName] = section
}

func envPartToKey(part string) string {
	part = strings.TrimSpace(part)
	if part == "" {
		return ""
	}
	return strings.ToLower(strings.ReplaceAll(part, "_", "-"))
}

func (s *Section) setPath(path []string, value any) {
	if s == nil || len(path) == 0 {
		return
	}
	if s.data == nil {
		s.data = map[string]any{}
	}
	current := s.data
	for i := 0; i < len(path)-1; i++ {
		key := path[i]
		if key == "" {
			return
		}
		next, ok := current[key]
		if !ok {
			child := map[string]any{}
			current[key] = child
			current = child
			continue
		}
		switch typed := next.(type) {
		case map[string]any:
			current = typed
		case Section:
			if typed.data == nil {
				typed.data = map[string]any{}
			}
			current[key] = typed.data
			current = typed.data
		case *Section:
			if typed == nil {
				child := map[string]any{}
				current[key] = child
				current = child
				continue
			}
			if typed.data == nil {
				typed.data = map[string]any{}
			}
			current[key] = typed.data
			current = typed.data
		default:
			child := map[string]any{}
			current[key] = child
			current = child
		}
	}
	leaf := path[len(path)-1]
	if leaf == "" {
		return
	}
	current[leaf] = value
}
