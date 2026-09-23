package app

import (
	"github.com/linxlib/fw/v2/config"
)

// EngineConfig 是 fw 引擎的配置根, 通过 config.Config.LoadByTags 按 inject tag 注入.
//
// 每个字段同时带 yaml tag(配置文件里的段名)与 inject tag(配置 key), 段缺失时保留
// default tag 给出的缺省值, 因此配置文件可以只写关心的部分.
type EngineConfig struct {
	Server      ServerConfig              `yaml:"server"      inject:"server"`
	Log         LogConfig                 `yaml:"log"         inject:"log"`
	OpenAPI     OpenAPIConfig             `yaml:"openapi"     inject:"openapi"`
	Recovery    RecoveryConfig            `yaml:"recovery"    inject:"recovery"`
	ProjectDir  string                    `yaml:"project_dir" inject:"project_dir"`
	Middlewares map[string]config.Section `yaml:"middlewares" inject:"middlewares"`
}

// ServerConfig 监听地址配置.
type ServerConfig struct {
	Host string `yaml:"host" inject:"host" default:"\"0.0.0.0\""`
	Port int    `yaml:"port" inject:"port" default:"8080"`
}

// LogConfig 日志配置.
type LogConfig struct {
	Level          string `yaml:"level"            inject:"level"             default:"\"info\""`
	Output         string `yaml:"output"           inject:"output"            default:"\"console\""`
	FilePath       string `yaml:"file_path"        inject:"file_path"         default:"\"\""`
	EnableFile     bool   `yaml:"enable_file"      inject:"enable_file"       default:"false"`
	RequestEnabled bool   `yaml:"request_enabled"  inject:"request_enabled"   default:"true"`
}

// OpenAPIConfig OpenAPI 文档生成配置.
type OpenAPIConfig struct {
	Enabled bool   `yaml:"enabled" inject:"enabled" default:"true"`
	Output  string `yaml:"output"  inject:"output"  default:"\"openapi.json\""`
	Title   string `yaml:"title"   inject:"title"   default:"\"fw API\""`
	Version string `yaml:"version" inject:"version" default:"\"1.0.0\""`
}

// RecoveryConfig panic 恢复配置.
type RecoveryConfig struct {
	Enabled           bool `yaml:"enabled"               inject:"enabled"                default:"true"`
	ReturnStackToBody bool `yaml:"return_stack_to_body"  inject:"return_stack_to_body"   default:"false"`
}

// DefaultEngineConfig 返回引擎配置的缺省值(等价于配置文件全部缺省时的结果).
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		Server:   ServerConfig{Host: "0.0.0.0", Port: 8080},
		Log:      LogConfig{Level: "info", Output: "console", RequestEnabled: true},
		OpenAPI:  OpenAPIConfig{Enabled: true, Output: "openapi.json", Title: "fw API", Version: "1.0.0"},
		Recovery: RecoveryConfig{Enabled: true},
	}
}
