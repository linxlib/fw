package options

import (
	"github.com/jinzhu/configor"
	"github.com/linxlib/fw/internal"
	"net"
	"os"
)

type ServerOption struct {
	intranetIP            string
	Dev                   bool   `yaml:"dev" default:"true"`
	Debug                 bool   `yaml:"debug" default:"true"`
	NoColor               bool   `yaml:"nocolor" default:"false"`
	BasePath              string `yaml:"basePath"`
	Listen                string `yaml:"listen" default:"127.0.0.1"` //监听地址
	Name                  string `yaml:"name" default:"fw"`          //server_token
	ShowRequestTimeHeader bool   `yaml:"showRequestTimeHeader,omitempty" default:"true"`
	RequestTimeHeader     string `yaml:"requestTimeHeader,omitempty" default:"Request-Time"`
	Port                  int    `yaml:"port" default:"2024"`
	AstFile               string `yaml:"astFile" default:"gen.json"` //ast json file generated by github.com/linxlib/astp. default is gen.json
}

func ReadConfig(o *ServerOption) {
	o.intranetIP = getIntranetIP()
	err := configor.New(&configor.Config{
		AutoReload: true,
		ENVPrefix:  "FW",
	}).Load(o, "config/config.yaml")
	if err != nil {
		panic(err)
		return
	}

}

// TODO: 更坚挺，特别是有许多网卡时（包括虚拟网卡）
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
