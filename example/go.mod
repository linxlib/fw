module github.com/linxlib/fw/example

go 1.23.0

toolchain go1.24.1

require (
	github.com/linxlib/astp v0.3.2
	github.com/linxlib/fw v0.0.0-20240731044621-6e5bf246209e
)

require (
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/fasthttp/router v1.5.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gookit/color v1.5.4 // indirect
	github.com/gookit/filter v1.2.2 // indirect
	github.com/gookit/goutil v0.6.18 // indirect
	github.com/gookit/validate v1.5.5 // indirect
	github.com/jinzhu/configor v1.2.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/linxlib/conv v1.1.1 // indirect
	github.com/linxlib/inject v0.1.3 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/savsgio/gotils v0.0.0-20250408102913-196191ec6287 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.62.0 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/linxlib/astp => ../../../linxlib/astp
	github.com/linxlib/fw => ..
)
