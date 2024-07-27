package fw

import (
	"github.com/linxlib/config"
	"github.com/linxlib/fw/internal"
	"gopkg.in/yaml.v2"
	"net"
	"os"
	"time"
)

type Option struct {
	intranetIP string `yaml:"intranetIP"`
	y          *config.YAML
	Server     ServerOption
}

type StaticDir struct {
	Path string `yaml:"path"`
	Root string `yaml:"root"`
}

type ServerOption struct {
	BasePath              string      `yaml:"basePath"`
	Listen                string      `yaml:"listen"` //监听地址
	Name                  string      `yaml:"name"`   //server_token
	ShowRequestTimeHeader bool        `yaml:"showRequestTimeHeader"`
	Key                   string      `yaml:"key"`  //ssl key
	Cert                  string      `yaml:"cert"` //ssl cert
	Port                  int         `yaml:"port"`
	StaticDirs            []StaticDir `yaml:"staticDirs"`
}

var _defaultServerOption = ServerOption{
	BasePath:              "",
	Listen:                "0.0.0.0",
	Port:                  time.Now().Year(),
	ShowRequestTimeHeader: true,
	StaticDirs: []StaticDir{
		{Path: "/static", Root: "example/chat"},
	},
}

func writeConfig(o *Option) {
	if !internal.FileIsExist("config/config.yaml") {
		bs, _ := yaml.Marshal(o)
		internal.WriteFile("config/config.yaml", bs, true)
	}
}

func readConfig(o *Option) *Option {
	if internal.FileIsExist("config/config.yaml") {
		conf, err := config.NewYAML(config.File("config/config.yaml"))
		if err != nil {
			internal.Errorf("%s", err)
		}
		err = conf.Get("server").Populate(&_defaultServerOption)
		if err != nil {
			internal.Errorf("%s", err)
		}
		o.y = conf
	} else {
		internal.Warnf("file %s not exist, use default options", "config/config.yaml")
	}
	o.Server = _defaultServerOption
	return o
}
func getIntranetIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		internal.Errorf("%s", err)
		os.Exit(1)
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}

		}
	}
	return "localhost"
}
func defaultOption() *Option {
	o := &Option{
		intranetIP: getIntranetIP(),
	}
	return readConfig(o)
}
