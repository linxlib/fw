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
