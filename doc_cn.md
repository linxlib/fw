# FW 框架文档

## 概述

FW 是一个面向 API 快速开发的轻量级 Go Web 框架。它基于 `fasthttp`，用注释注解的方式定义路由和中间件，并内置了 AST 解析、依赖注入、统一响应、OpenAPI 生成和脚手架工具。

模块路径：

```go
github.com/linxlib/fw/v2
```

运行时依赖：

- `github.com/valyala/fasthttp`
- `github.com/fasthttp/router`
- `github.com/pterm/pterm`
- `gopkg.in/yaml.v3`

## 主要特性

- 基于注解的路由注册
- `Controller`、`Service`、`Middleware` 自动注册
- 全局、控制器级、方法级中间件
- 确定性的中间件合并与执行顺序
- 请求级依赖注入
- 自动绑定路径、查询、请求头、请求体参数
- 更易用的请求上下文封装
- 自动序列化方法返回值，并提供统一 JSON 响应结构，可替换格式化器和编码器
- Panic 恢复与请求日志
- 自动生成 OpenAPI 文档和 Swagger UI
- 提供 `cmd/fw` CLI 用于初始化项目、构建和生成代码骨架

## 快速开始

### 1. 初始化项目

```bash
fw init demo
cd demo
```

`fw init` 默认会创建：

- `main.go`
- `controllers/`
- `services/`
- `middlewares/`
- `models/`
- `config/app.yaml`
- `.gitignore`

生成的 `go.mod` 模块名默认等于项目名或目录名。如果你希望使用 `github.com/you/demo` 这种完整模块路径，需要在初始化后自行调整 `go.mod`。

### 2. 生成元数据并构建

```bash
fw build
```

`fw build` 会执行三步：

1. 运行 `go generate ./...`
2. 生成 `.astp.json` 和 `fw_autoreg_gen.go`
3. 编译二进制，并输出产物大小和 SHA-256

### 3. 开发运行

```bash
go run .
```

注意：脚手架生成的项目会用 `go:embed` 嵌入 `.astp.json`，所以第一次运行前，建议先执行一次 `fw build` 或 `go generate ./...`。

## 基础目录结构

- `app/`：引擎生命周期、控制器注册、路由构建、运行时分发
- `annotation/`：注解统一读取
- `astp/`：AST 解析和自动注册代码生成
- `inject/`：依赖注入容器和调用解析
- `config/`：YAML 配置和环境变量覆盖
- `logger/`：彩色控制台日志和文件日志
- `context/`：请求上下文包装
- `middleware/`：中间件接口和执行链
- `response/`：默认响应结构、格式化器、编码器
- `openapi/`：OpenAPI 生成和文档路由
- `cmd/example/`：可运行示例
- `cmd/fw/`：CLI 工具

## 框架启动流程

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

`Engine` 启动时主要会做这些事：

1. 加载配置
2. 从源码、`.astp.json` 或嵌入数据中加载项目元信息
3. 应用 ASTP 生成的自动注册器
4. 收集路由和中间件绑定关系
5. 注册 HTTP 处理函数
6. 在启用时生成 `openapi.json` 并挂载文档路由

## 注解系统

FW 使用 Go 注释中的注解，例如 `// @GET /users`、`// @Authorization(Admin)`。

支持的写法：

- `@X`
- `@X value`
- `@X(a,b)`
- `@X(k=v, role=admin)`

和响应相关的框架注解还包括：

- `@Response(...)`：指定哪个返回值用于 OpenAPI 响应结构推断
- `@RawResponse`：让第一个非 `error` 返回值直接输出，不再包裹默认的 `code/message/data`

## Controller 用法

使用 `@Controller` 声明控制器，使用 `@Route` 定义基础路由。

```go
package controllers

import (
	"github.com/linxlib/fw/v2/context"
	"github.com/valyala/fasthttp"
)

// UserController 用户接口控制器
// @Controller
// @Route /api/v1/users
type UserController struct{}

// GetUser 获取单个用户
// @GET /:id
func (c *UserController) GetUser(ctx context.Context, id string) error {
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", map[string]any{
		"id": id,
	})
}
```

规则说明：

- `@Route /base` 或 `@Route(/base)` 定义控制器基础路径
- 方法上的 HTTP 注解会拼接到基础路径下
- 支持的 HTTP 注解有 `@GET`、`@POST`、`@PUT`、`@DELETE`、`@PATCH`、`@OPTIONS`、`@HEAD`
- 一个方法可以声明多个相同 HTTP 方法的路由，只要 URL 不同即可
- 重复的 `方法 + 路径` 会被拒绝注册

示例：

```go
// @POST /user
// @POST /user_info
func (c *UserController) ModifyUser(...) {}
```

## Service 用法

给结构体加上 `@Service` 后，它就会被自动注册生成器识别。

```go
package services

// UserService 业务服务
// @Service
type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}
```

当生成器扫描到 `@Service` 时：

- 如果存在无参构造函数 `New<TypeName>()`，优先调用它
- 否则会使用 `&TypeName{}`
- 注册结果会被映射到全局注入容器中，供控制器和处理函数注入

如果你想让生成器跳过某个 `Service`、`Controller` 或 `Middleware`，可以加上 `@Disable`。

## Middleware 用法

### 定义中间件

```go
package middlewares

import (
	"github.com/linxlib/fw/v2/context"
	"github.com/linxlib/fw/v2/middleware"
)

// AuthMiddleware 鉴权中间件
// @Middleware
// @Global
type AuthMiddleware struct{}

func (AuthMiddleware) Spec() middleware.AnnotationSpec {
	return middleware.AnnotationSpec{
		Name:  "Auth",
		Scope: middleware.ScopeBoth,
		Stage: middleware.StageBefore,
	}
}

func (AuthMiddleware) Handle(ctx context.Context, args middleware.AnnotationArgs, next middleware.Handler) error {
	return next(ctx)
}
```

### 中间件作用域

- `middleware.ScopeController`：只能标注在控制器上
- `middleware.ScopeMethod`：只能标注在方法上
- `middleware.ScopeBoth`：控制器和方法上都能用

### 中间件阶段

- `middleware.StageBefore`：只做前置逻辑
- `middleware.StageAfter`：只做后置逻辑
- `middleware.StageBoth`：前后都执行

### 执行顺序

执行顺序是固定的：

1. 全局前置
2. 控制器前置
3. 方法前置
4. Handler
5. 方法后置
6. 控制器后置
7. 全局后置

### 合并与覆盖规则

- 中间件通过注解名匹配
- 同一层级上，同名注解后者覆盖前者
- 方法级中间件会覆盖同名控制器级中间件
- 不同名字的中间件会合并执行

### 忽略中间件

可以在控制器方法上使用 `@Ignore(...)` 跳过中间件。

```go
// @GET /health
// @Ignore(Auth,Global)
func (c *UserController) Health(ctx context.Context) error {
	return ctx.Respond(fasthttp.StatusOK, 0, "healthy", nil)
}
```

说明：

- `@Ignore(Auth)`：忽略指定中间件
- `@Ignore(Global)`：忽略所有全局中间件
- 多个 `@Ignore(...)` 会自动合并

### `@Global` 的真实作用

`@Global` 主要给自动注册生成器使用。一个中间件同时拥有 `@Middleware` 和 `@Global` 时，生成代码会调用 `e.Use(...)`。如果没有 `@Global`，生成代码会调用 `e.RegisterMiddleware(...)`，这类中间件只有在注解中显式声明时才会运行。

### 在中间件中读取注解参数

框架会把注解参数传给 `middleware.AnnotationArgs`。

```go
// @Auth(role=admin)
```

在中间件中读取：

```go
func (AuthMiddleware) Handle(ctx context.Context, args middleware.AnnotationArgs, next middleware.Handler) error {
	role := args.KV["role"]
	_ = role
	return next(ctx)
}
```

### OpenAPI 安全集成

如果中间件实现了 `middleware.SecurityProvider`，框架会自动把它转换成 OpenAPI 的安全方案，并在受保护的接口上标记锁图标。

## 依赖注入

FW 有两层注入容器：

- 全局容器：应用级服务和共享对象
- 请求容器：每次请求独立的值和请求级绑定

### 全局注入

你可以手动注册依赖：

```go
e.Container().Map(NewUserService())
```

控制器注册时，框架还会自动给零值字段做一次全局容器注入。

### 结构体字段注入

注入器支持带 `inject` 标签的结构体字段注入。

```go
type MyHandler struct {
	Service *UserService `inject:""`
}
```

### 处理函数参数注入

请求处理时，方法参数可以自动从以下来源解析：

- FW 自己的 `context.Context`
- `*context.FWContext`
- `*fasthttp.RequestCtx`
- 注入容器里的服务对象
- 路径、查询、请求头中的基础类型
- 从 body、query、path、header 填充的结构体或结构体指针

示例：

```go
func (c *UserController) Create(ctx context.Context, body CreateUserBody, svc *UserService) error {
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", svc.Create(body))
}
```

## 参数绑定

FW 会自动推断参数来源，同时也支持通过注解和标签显式指定。

### 自动推断规则

每个方法参数会按下面顺序判断：

1. 如果参数类型上有 `@Query`、`@Path`、`@Header`、`@Body`，优先使用该来源
2. 否则，如果类型名以 `Query`、`Path`、`Header`、`Body` 结尾，也会作为提示
3. 否则，如果参数名命中路由参数，例如 `:id`，则视为路径参数
4. 否则，如果是已解析出的结构体类型，默认从 body 绑定
5. 其他基础类型默认从 query 绑定

### 基础类型绑定

基础类型参数支持从字符串自动解析到：

- `string`
- `bool`
- 有符号整数
- 无符号整数
- 浮点数
- 上述类型的指针

示例：

```go
// @GET /users/:id
func (c *UserController) Detail(ctx context.Context, id int, verbose bool) error {
	// id 来自 path
	// verbose 来自 query
	return nil
}
```

### 结构体绑定

结构体和结构体指针可以从以下位置填充：

- JSON 请求体
- 查询参数
- 路径参数
- 请求头

支持的标签：

- `query:"name"`
- `path:"id"`
- `header:"x-trace-id"`
- `json:"name"`

Body 示例：

```go
// CreateUserBody 请求体
// @Body
type CreateUserBody struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}
```

```go
func (c *UserController) Create(ctx context.Context, body CreateUserBody) error {
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", body)
}
```

Query 示例：

```go
// ListUserQuery 查询模型
// @Query
type ListUserQuery struct {
	Keyword string `query:"keyword"`
	Page    int    `query:"page"`
}
```

```go
func (c *UserController) List(ctx context.Context, query ListUserQuery) error {
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", query)
}
```

## Context API

FW 对 `*fasthttp.RequestCtx` 做了更友好的封装。

常用方法包括：

- `Raw()`
- `Method()`
- `Scheme()`
- `Host()`
- `Path()`
- `URI()`
- `Header(name)`
- `Headers()`
- `Param(name)`
- `Params()`
- `Query(name)`
- `Queries()`
- `Body()`
- `BindJSON(out)`
- `Respond(statusCode, code, message, data)`
- `RawResponse()`
- `StatusOK(message ...)`
- `StatusParamError(message ...)`
- `StatusServerError(message ...)`
- `Set(key, value)`
- `Get(key)`
- `MustGet(key)`
- `GetString(key)`
- `GetInt(key)`
- `Container()`
- `TraceID()`
- `SSE(...)`

## SSE 和 WebSocket 用法

示例项目展示了“方法级中间件接管请求流程”的两种典型写法。

### SSE

`SSE` 中间件会开启 Server-Sent Events 流，并把 `*context.SSEWriter` 映射到请求容器中，因此处理函数可以直接把它作为参数接收。

```go
// @GET /stream
// @SSE
func (c *UserController) Stream(ctx context.Context, w *context.SSEWriter) error {
	return nil
}
```

### WebSocket

`WS` 中间件会升级连接、读取消息、把当前消息映射到请求容器中，并交由处理函数生成回复。

```go
// @GET /ws
// @WS
func (c *UserController) Echo(ctx context.Context, msg []byte) error {
	ctx.Set("WS.Reply", msg)
	return nil
}
```

## 响应模型

默认响应结构：

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "trace_id": "..."
}
```

### 普通返回

常见场景推荐使用链式响应 builder：

```go
return ctx.StatusOK().Data(data)
```

例如：

```go
return ctx.StatusOK().Message("saved").Data(result)
return ctx.StatusOK("healthy").Send()
return ctx.StatusParamError("param xx invalid").Send()
return ctx.StatusServerError().Message("").Send()
```

如果你希望完全手动指定 `statusCode/code/message/data`，仍然可以继续使用 `Respond(...)`。

### 根据方法返回值自动输出响应

如果处理函数返回了值，并且它自己没有主动写响应，FW 会把第一个非 `error` 返回值当作成功响应数据。

```go
// @GET /profile
func (c *UserController) Profile() map[string]any {
	return map[string]any{"name": "alice"}
}
```

默认仍然会包裹成统一结构：

```json
{"code":0,"message":"ok","data":{"name":"alice"}}
```

如果方法返回了 `error`，且该 `error` 不为 `nil`，FW 会把它视为失败，不再输出成功响应。

### 使用 `@RawResponse` 输出原始响应

当你希望方法返回值直接作为响应体输出时，可以在方法上标记 `@RawResponse`。

```go
// @GET /profile/raw
// @RawResponse
func (c *UserController) ProfileRaw() map[string]any {
	return map[string]any{"name": "alice"}
}
```

响应体会变成：

```json
{"name":"alice"}
```

说明：

- `@RawResponse` 只作用于方法级
- 它只影响框架根据返回值自动写响应的行为
- 如果方法或中间件已经写过响应，FW 不会再覆盖
- `ctx.Respond(...)` 和 `ctx.StatusOK()` 这类方法仍然会走当前配置的格式化器，保留默认包裹结构

### 使用 `ctx.RawResponse()` 手动输出原始响应

如果你希望在处理函数或中间件里直接输出原始响应体，而不是默认包裹结构，可以使用 `ctx.RawResponse()`。

```go
return ctx.RawResponse().Data("ok")
```

响应体会变成：

```json
"ok"
```

说明：

- `ctx.RawResponse()` 用于手动写响应
- `@RawResponse` 用于方法返回值的自动响应
- 两者仍然会走当前配置的编码器

### 默认空成功响应

如果处理函数结束时没有主动写出响应，也没有返回错误，FW 会自动输出：

```json
{"code":0,"message":"ok"}
```

### 自定义格式化器和编码器

你可以替换响应格式化器和编码器：

```go
e.SetResponse(customFormatter, customEncoder)
```

## OpenAPI 和 Swagger

当 `openapi.enabled` 为 `true` 时，框架启动时会自动生成 OpenAPI 文件并挂载文档路由。

默认文档路由：

- `/docs`
- `/docs/config`
- `/docs/openapi.json`

### OpenAPI 会生成哪些内容

- 路径和 HTTP 方法
- Operation ID
- 控制器名作为 tag
- 方法注释作为 summary
- path、query、header、body 参数结构
- 基础 `200` 响应结构
- 来自中间件的安全定义

### 返回结构推断规则

默认情况下，FW 会把方法返回值中第一个非 `error` 的结果当作 OpenAPI 里的 `data` 结构。

你也可以通过 `@Response` 指定：

```go
// @Response(index=1)
```

支持的写法：

- `@Response(1)`
- `@Response(index=1)`
- `@Response(result=1)`

正整数索引按 1 开始计数。

如果方法同时标记了 `@RawResponse`，那么 OpenAPI 会直接把选中的返回值作为顶层 `200` 响应结构，而不是再放到 `data` 字段下面。

## 配置

配置加载顺序：

1. 框架默认值
2. YAML 文件
3. `FW_` 前缀环境变量

配置示例：

```yaml
project_dir: .

server:
  host: "0.0.0.0"
  port: 8080

log:
  level: info
  output: console
  file_path: logs/fw.log
  enable_file: false
  request_enabled: true

recovery:
  enabled: true
  return_stack_to_body: true

openapi:
  enabled: true
  output: openapi.json
  title: "FW API"
  version: "1.0.0"

middlewares:
  authorization:
    api-key: xxxx
```

### 常用环境变量覆盖

```bash
FW_SERVER_HOST=127.0.0.1
FW_SERVER_PORT=9090
FW_LOG_LEVEL=debug
FW_OPENAPI_ENABLED=false
```

### 用环境变量覆盖中间件配置

中间件配置也支持环境变量覆盖。

例如：

```bash
FW_MIDDLEWARES_AUTHORIZATION_API_KEY=secret
```

等价于：

```yaml
middlewares:
  authorization:
    api-key: secret
```

## Recovery 和日志

### Panic 恢复

默认开启 Recovery。

当请求过程中发生 panic 时：

- 框架会记录 panic 和格式化后的调用栈
- 返回 HTTP `500`
- 输出统一错误响应
- 可选地把栈信息返回到响应体

配置：

```yaml
recovery:
  enabled: true
  return_stack_to_body: true
```

### 请求日志

当 `log.request_enabled` 为 `true` 时，框架会输出每个请求的方法、路径、状态码和耗时。

## 手动注册模式

如果你不想依赖自动生成的注册代码，也可以手动注册组件。

```go
e, _ := app.New("config/app.yaml")

e.Container().Map(services.NewUserService())
e.Use(&middlewares.AuthMiddleware{})
e.RegisterMiddleware(&middlewares.LogMiddleware{})
e.RegisterController(&controllers.UserController{})
```

## `cmd/fw` CLI 用法

框架自带 `cmd/fw` 工具。

在当前仓库直接运行：

```bash
go run ./cmd/fw --help
```

也可以安装到本地：

```bash
go install github.com/linxlib/fw/v2/cmd/fw@latest
```

### `fw init`

创建新项目：

```bash
fw init myapp
```

或者在当前目录初始化：

```bash
fw init
```

说明：

- 不带项目名时，要求当前目录基本为空
- 生成的模块名默认等于目录名或项目名

### `fw build`

构建当前项目：

```bash
fw build
```

构建其它目录：

```bash
fw build ./cmd/server
fw build --dir ./cmd/server
```

交叉编译：

```bash
fw build -os linux -arch amd64
fw build -os macos -arch arm64 -o app
fw build -os windows -arch amd64 -o demo.exe
```

参数说明：

- `-o, --output`：输出文件名
- `--os`：目标系统，支持 `windows`、`linux`、`macos`
- `--arch`：目标架构，支持 `amd64`、`arm64`
- `--dir`：工作目录

行为细节：

- 目标系统是 Windows 时，如果输出名没有 `.exe` 会自动补上
- 非 Windows 目标会自动移除输出名中的 `.exe`
- 交叉编译时会设置 `CGO_ENABLED=0`

### `fw create`

生成框架组件骨架：

```bash
fw create controller --name User --route /api/v1/users
fw create service --name User
fw create middleware --name Authorization
```

可用子命令：

- `fw create controller`
- `fw create service`
- `fw create middleware`

说明：

- 不传 `--name` 时会进入交互式输入
- 名称必须以字母开头，且只能包含字母和数字
- 文件会分别生成到 `controllers/`、`services/`、`middlewares/`
- 已存在文件不会被覆盖

### `fw version`

```bash
fw version
```

### `fw completion`

因为 CLI 基于 Cobra，所以也可以使用自动补全命令：

```bash
fw completion bash
fw completion zsh
```

## 推荐开发流程

1. 执行 `fw init your-app`
2. 编写控制器、服务和中间件
3. 给结构体加上 `@Controller`、`@Service`、`@Middleware`
4. 运行 `fw build`，重新生成 AST 元数据和自动注册文件
5. 开发时使用 `go run .`
6. 打开 `/docs` 查看自动生成的接口文档
7. 交付前执行 `go test ./...`

## 一个包含多种能力的示例

```go
package controllers

import (
	"github.com/your/mod/models"
	"github.com/your/mod/services"
	"github.com/linxlib/fw/v2/context"
	"github.com/valyala/fasthttp"
)

// UserController 用户接口
// @Controller
// @Route /api/v1/users
// @Authorization(Admin)
type UserController struct {
	Service *services.UserService
}

// Update 更新用户
// @POST /:id
// @Log(stage=before)
func (c *UserController) Update(ctx context.Context, id int, body models.UpdateUserBody) error {
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", map[string]any{
		"id":   id,
		"body": body,
	})
}

// Health 健康检查
// @GET /health
// @Ignore(Authorization,Global)
func (c *UserController) Health(ctx context.Context) error {
	return ctx.Respond(fasthttp.StatusOK, 0, "healthy", nil)
}
```

## 总结

FW 适合这样的场景：

- 希望快速开发 API，减少样板代码
- 喜欢用注解描述路由和中间件
- 需要自动依赖注入和参数绑定
- 想自动生成 OpenAPI 文档
- 需要一个轻量、可控、内聚的开发工具链
