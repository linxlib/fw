package config

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
)

// getENVPrefix 返回环境变量前缀, ENVPrefix 为 "-" 时表示仅使用显式 env tag, 不做前缀派生.
func (c *Config) getENVPrefix() string {
	if c.ENVPrefix != "" {
		return c.ENVPrefix
	}
	return "FW"
}

// candidateEnvNames 生成字段的环境变量候选名(按优先级从高到低):
//  1. 显式 env tag(命中即只查它);
//  2. 前缀派生: <ENVPrefix>_<KEYPATH大写>_<PREFIX链>_<FIELD大写> (key 的 "." 替换为 "_", ENVPrefix 为 "-" 时跳过);
//  3. 无前缀派生: <KEYPATH大写>_<PREFIX链>_<FIELD大写>.
//
// 内嵌结构体带 anonymous:"true" 时, 其字段路径不包含结构体名.
func (c *Config) candidateEnvNames(key string, sf reflect.StructField, prefixes []string) []string {
	if env := sf.Tag.Get("env"); env != "" {
		return []string{env}
	}
	// prefixes 是祖先结构体名链(不含当前字段), 当前字段名只追加一次
	chain := prefixes
	if key != "" {
		chain = append([]string{strings.ReplaceAll(strings.ToUpper(key), ".", "_")}, chain...)
	}
	chain = append(append([]string{}, chain...), strings.ToUpper(sf.Name))
	var names []string
	if p := c.getENVPrefix(); p != "" && p != "-" {
		names = append(names, strings.ToUpper(strings.Join(append([]string{p}, chain...), "_")))
	}
	names = append(names, strings.ToUpper(strings.Join(chain, "_")))
	return names
}

// getPrefixForStruct 处理 anonymous 内嵌结构体的前缀传递.
func getPrefixForStruct(prefixes []string, sf reflect.StructField) []string {
	if sf.Anonymous && sf.Tag.Get("anonymous") == "true" {
		return prefixes
	}
	return append(prefixes, sf.Name)
}

// processEnv 按环境变量覆盖 config 中的字段, env 优先级高于 yaml.
// key 为 target 注册时的 key(点号路径), 用于派生候选变量名.
func (c *Config) processEnv(config any, key string) error {
	return c.applyEnv(reflect.Indirect(reflect.ValueOf(config)), key, nil)
}

func (c *Config) applyEnv(v reflect.Value, key string, prefixes []string) error {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		f := v.Field(i)
		if !f.CanAddr() || !f.CanInterface() {
			continue
		}
		for _, name := range c.candidateEnvNames(key, sf, prefixes) {
			if value := envLookup(name); value != "" {
				if c.Debug || c.Verbose {
					fmt.Printf("config: loading field %s from env %s\n", sf.Name, name)
				}
				if err := setFieldFromEnv(f, value); err != nil {
					return err
				}
				break
			}
		}
		inner := f
		for inner.Kind() == reflect.Pointer {
			inner = inner.Elem()
		}
		switch inner.Kind() {
		case reflect.Struct:
			if err := c.applyEnv(inner, key, getPrefixForStruct(prefixes, sf)); err != nil {
				return err
			}
		case reflect.Slice:
			for j := 0; j < inner.Len(); j++ {
				if reflect.Indirect(inner.Index(j)).Kind() == reflect.Struct {
					idxPrefixes := append(append([]string{}, prefixes...), fmt.Sprint(j))
					if err := c.applyEnv(reflect.Indirect(inner.Index(j)), key, getPrefixForStruct(idxPrefixes, sf)); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// setFieldFromEnv 把环境变量值写入字段: bool 宽松解析, string 直接赋值,
// 其他类型按 YAML 字面量解码.
func setFieldFromEnv(f reflect.Value, value string) error {
	switch reflect.Indirect(f).Kind() {
	case reflect.Bool:
		switch strings.ToLower(value) {
		case "0", "f", "false", "":
			f.Set(reflect.ValueOf(false))
		default:
			f.Set(reflect.ValueOf(true))
		}
	case reflect.String:
		f.Set(reflect.ValueOf(value))
	default:
		if err := yamlUnmarshal([]byte(value), f.Addr().Interface()); err != nil {
			return fmt.Errorf("config: set field %s from env value %q: %w", f.Type(), value, err)
		}
	}
	return nil
}

// lookupEnv 是 os.Getenv 的可替换入口(测试用).
func lookupEnv(name string) string {
	return os.Getenv(name)
}

var testRegexp = regexp.MustCompile(`_test|\.test$`)

// detectTestBinary 判断当前是否在 go test 进程中运行.
func detectTestBinary() bool {
	if len(os.Args) == 0 {
		return false
	}
	return testRegexp.MatchString(os.Args[0])
}
