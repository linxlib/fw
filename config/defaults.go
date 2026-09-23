package config

import (
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"
)

// processDefaults 递归填充 default tag: 字段为零值时, 用 default tag 的 YAML 字面量解码填充.
// 缺 section 或重载后字段被移除时, 字段会先被归零, 再由这里恢复默认值.
func (c *Config) processDefaults(config any) error {
	v := reflect.Indirect(reflect.ValueOf(config))
	if v.Kind() != reflect.Struct {
		return nil // 非结构体目标(如点号路径注入的普通变量)无需默认值处理
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		f := v.Field(i)
		if !f.CanAddr() || !f.CanInterface() {
			continue
		}
		if def := sf.Tag.Get("default"); def != "" && reflect.DeepEqual(f.Interface(), reflect.Zero(f.Type()).Interface()) {
			if err := yamlUnmarshal([]byte(def), f.Addr().Interface()); err != nil {
				return fmt.Errorf("config: field %s default %q: %w", sf.Name, def, err)
			}
		}
		inner := f
		for inner.Kind() == reflect.Pointer {
			inner = inner.Elem()
		}
		switch inner.Kind() {
		case reflect.Struct:
			if err := c.processDefaults(inner.Addr().Interface()); err != nil {
				return err
			}
		case reflect.Slice:
			for j := 0; j < inner.Len(); j++ {
				if reflect.Indirect(inner.Index(j)).Kind() == reflect.Struct {
					if err := c.processDefaults(inner.Index(j).Addr().Interface()); err != nil {
						return err
					}
				}
			}

		default:
			// reflect.Kind 的其余取值刻意不处理: default tag 只对叶子字段有意义,
			// 且上方已按字段写入; 这里只递归下钻结构体与切片, 无需处理其他 Kind.
		}
	}
	return nil
}

// processRequired 校验 required:"true" 的字段, 缺失时报错.
func (c *Config) processRequired(config any) error {
	return c.checkRequired(reflect.Indirect(reflect.ValueOf(config)))
}

func (c *Config) checkRequired(v reflect.Value) error {
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
		if sf.Tag.Get("required") == "true" && reflect.DeepEqual(f.Interface(), reflect.Zero(f.Type()).Interface()) {
			return fmt.Errorf("config: field %s is required, but blank", sf.Name)
		}
		inner := f
		for inner.Kind() == reflect.Pointer {
			inner = inner.Elem()
		}
		switch inner.Kind() {
		case reflect.Struct:
			if err := c.checkRequired(inner); err != nil {
				return err
			}
		case reflect.Slice:
			for j := 0; j < inner.Len(); j++ {
				if err := c.checkRequired(reflect.Indirect(inner.Index(j))); err != nil {
					return err
				}
			}

		default:
			// reflect.Kind 的其余取值刻意不处理: required 只校验叶子字段,
			// 这里只递归下钻结构体与切片, 无需处理其他 Kind.
		}
	}
	return nil
}

// yamlUnmarshal 是 yaml.Unmarshal 的可替换入口(测试用).
var yamlUnmarshal = yaml.Unmarshal
