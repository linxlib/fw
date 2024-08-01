package options

import (
	"github.com/jinzhu/configor"
	"net"
)

type ServerOption struct {
	intranetIP            string
	Dev                   bool         `yaml:"dev" default:"true"`
	Debug                 bool         `yaml:"debug" default:"true"`
	NoColor               bool         `yaml:"nocolor" default:"false"`
	BasePath              string       `yaml:"basePath" default:"/"`
	Listen                string       `yaml:"listen" default:"127.0.0.1"` //监听地址
	Title                 string       `yaml:"title" default:"fw api"`
	Name                  string       `yaml:"name" default:"fw"` //server_token
	ShowRequestTimeHeader bool         `yaml:"showRequestTimeHeader,omitempty" default:"true"`
	RequestTimeHeader     string       `yaml:"requestTimeHeader,omitempty" default:"Request-Time"`
	Port                  int          `yaml:"port" default:"2024"`
	AstFile               string       `yaml:"astFile" default:"gen.json"` //ast json file generated by github.com/linxlib/astp. default is gen.json
	Logger                LoggerOption `yaml:"logger"`
}
type LoggerOption struct {
	LoggerLevel       int    `yaml:"loggerLevel" default:"4"` //0-6 0: Panic 6: Trace
	SeparateLevelFile bool   `yaml:"separateLevelFile" default:"false"`
	LogDir            string `yaml:"logDir" default:"log"`
	RotateFile        bool   `yaml:"rotate" default:"true"`
	MaxSize           int    `yaml:"maxSize" default:"5"`
	MaxAge            int    `yaml:"maxAge" default:"28"`
	MaxBackups        int    `yaml:"maxBackups" default:"3"`
	Compress          bool   `yaml:"compress" default:"false"`
	LocalTime         bool   `yaml:"localTime" default:"true"`
}

func ReadConfig(o *ServerOption) {
	o.intranetIP = getIntranetIP()
	err := configor.New(&configor.Config{
		AutoReload: true,
		ENVPrefix:  "FW",
	}).Load(o, "config/config.yaml")
	if err != nil {
		panic(err)
	}
}

func getIntranetIP() string {
	conn, err := net.Dial("udp", "114.114.114.114:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}