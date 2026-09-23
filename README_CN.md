# FW

[English README](./README.md)

FW 是一个轻量级 Go Web 框架，适合快速开发 API，核心能力包括注解式路由、中间件绑定、依赖注入以及自动生成 OpenAPI 文档。

## 特性

- 基于注解的路由定义，例如 `@GET /users`
- Controller、Service、Middleware 自动注册
- 支持全局、控制器级、方法级中间件
- 自动绑定 path、query、header、body 参数
- 统一 JSON 响应结构
- 自动生成 OpenAPI 和 Swagger UI
- 提供 `cmd/fw` CLI 用于初始化和构建
- 可选的配置热重载，按顶层 section 增量生效
- 泛型 Controller 与方法，并据类型实参推断真实的 OpenAPI schema

## 安装

在项目中引入 FW：

```bash
go get github.com/linxlib/fw/v2
```

如果需要 CLI：

```bash
go install github.com/linxlib/fw/v2/cmd/fw@latest
```

**环境要求：** Go 1.27 及以上。从 **v2.1.0** 起 `go.mod` 声明 `go 1.27`，
更早的工具链会直接拒绝构建（`go.mod requires go >= 1.27`）。需要 1.27 是为了解析
Go 1.27 的方法泛型语法。

## 从 v2.0.x 及更早版本升级

v2.1.0 保持模块路径 `github.com/linxlib/fw/v2` 不变，升级只需 `go get`，无需改 import；
但如果你直接使用了 `config` 包，则**不兼容源码**。

### 必须改的代码

| v2.0.1 | v2.1.0 |
| --- | --- |
| `config.Load(path string) (Config, error)` | `config.Load(target any) error` |
| `config.Default()` | 已删除，改用 `app.DefaultEngineConfig()` |
| `config.ServerConfig` / `LogConfig` / `RecoveryConfig` / `OpenAPIConfig` | 移到 `app` 包 |
| `Config.Server` / `.Log` / `.OpenAPI` / `.Recovery` / `.ProjectDir` / `.Middlewares` | 已删除，`Config` 改为由 `config.New(&config.Option{...})` 构造的加载器 |
| `Section` 结构体及 `GetString` / `GetInt` / `GetBool` / `GetStrings` / `GetDuration` / `Sub` / `MustGet` / `UnmarshalYAML` | `Section` 即 `map[string]any`，只剩 `Get` 与 `Has` |

```go
// v2.0.1
cfg, err := config.Load("config/app.yaml")
host := cfg.Server.Host
ttl := mw.Config.GetDurationDefault("timeout", 5*time.Second)

// v2.1.0
type opt struct {
	Server struct {
		Host string `inject:"host" default:"\"0.0.0.0\""`
		Port int    `inject:"port" default:"8080"`
	} `inject:"server"`
}
c := config.New(&config.Option{Files: []string{"config/app.yaml"}})
var o opt
_ = c.LoadWithKey("server", &o) // 或 c.Load(&o) / c.LoadByTags(&o)

// 中间件配置：Section 就是普通 map，取值需自行断言
if mw.Config.Has("timeout") {
	d, _ := time.ParseDuration(mw.Config.Get("timeout"))
}
```

只用 `app.New("config/app.yaml")` 的话无需改代码。

### `openapi` 包

`RouteInfo`、`MediaType`、`Schema` 新增了字段（`ResponseExample`、`Example`、`Default`）。
不带字段名的位置化复合字面量将无法编译，需补上字段名。

### 编译通过但运行行为会变

- **环境变量派生规则改变。** 旧版按 `yaml` tag 派生（`FW_LOG_FILE_PATH`、
  `FW_RECOVERY_RETURN_STACK_TO_BODY`）；新版按 inject key + Go 字段名派生
  （`FW_LOG_FILEPATH`、`FW_RECOVERY_RETURNSTACKTOBODY`）。`FW_SERVER_HOST`、
  `FW_SERVER_PORT`、`FW_LOG_LEVEL`、`FW_OPENAPI_ENABLED` 不变。`FW_MIDDLEWARES_*`
  已失效，中间件配置改为只能写在 YAML 里。
- **配置文件缺失不再导致启动失败**，改为静默使用默认值；且只接受 `.yaml` 后缀。
- **默认值变化：** `log.file_path` 由 `logs/fw.log` 改为 `""`，
  `recovery.return_stack_to_body` 由 `true` 改为 `false`，
  `openapi.title` 由 `FW API` 改为 `fw API`。
- **同一中间件跨层绑定时只执行一次**，保留最具体的一层（method > controller > global）。
  去重是按**名字**而非实例，因此两个不同实现共用同一名字时，较不具体的那个不再执行。
- **集合路由去掉尾斜杠：** `@Route /users` + `@GET /` 现在映射到 `/users`。写死
  `/users/` 的客户端会 404；`/users` 则从 301 跳转转为直接响应。
- **重新生成 `.astp.json`**（`go generate ./...`）：若以内嵌 AST 元数据方式部署，
  v2.0.1 生成的文件缺少泛型基类控制器的路由。

### 顺带修掉的问题

- 缺省 query/header 参数回退零值，不再返回 500。
- 同类型形参不再互相串值（`?page=2&size=5`）。
- `:name` 路径参数真正可达——v2.0.1 上它们是静默 404 的，因为 fasthttp/router 只认 `{name}`。

## 快速开始

创建项目：

```bash
fw init demo
cd demo
fw build
go run .
```

典型启动代码：

```go
package main

import (
	_ "embed"

	"github.com/linxlib/fw/v2/app"
)

//go:generate go run github.com/linxlib/fw/v2/astp/cmd/astp -exported .

//go:embed .astp.json
var astpData []byte

func main() {
	e, err := app.New("config/app.yaml")
	if err != nil {
		panic(err)
	}

	e.EmbedProject(astpData)

	if err := e.ListenAndServe(); err != nil {
		panic(err)
	}
}
```

简单控制器示例：

```go
package controllers

import (
	"github.com/linxlib/fw/v2/context"
	"github.com/valyala/fasthttp"
)

// UserController 用户接口
// @Controller
// @Route /api/v1/users
type UserController struct{}

// Detail 获取单个用户
// @GET /:id
func (c *UserController) Detail(ctx context.Context, id string) error {
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", map[string]any{"id": id})
}
```

## 文档

- 总览文档：[doc.md](./doc.md)
- 英文完整文档：[doc_en.md](./doc_en.md)
- 中文完整文档：[doc_cn.md](./doc_cn.md)

给 coding agent 的文档：

- 应用开发指南：[APP_AGENT.md](./APP_AGENT.md)
- 文档维护指南：[DOCS_AGENT.md](./DOCS_AGENT.md)

## CLI

常用命令：

```bash
fw init myapp
fw build
fw create controller --name User --route /api/v1/users
fw create service --name User
fw create middleware --name Authorization
```

完整 CLI 说明请查看 [doc_en.md](./doc_en.md) 和 [doc_cn.md](./doc_cn.md)。

## License

[MIT](./LICENSE)
