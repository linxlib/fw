---
name: fw-app-development
description: 在基于 github.com/linxlib/fw/v2 的业务应用项目中开发功能时必须遵守的规范与工作流。涵盖新增 API 的标准流程、controllers/services/middlewares/models 分层职责、注解与参数绑定规则、响应规范、配置写法、元数据重新生成时机与交付前检查清单。当任务是在 fw 应用项目里加接口、改业务逻辑、加中间件或写配置时使用本 skill。注意：本 skill 不适用于修改 fw 框架源码本身。
---

# FW 应用开发规范

本 skill 面向**业务应用项目**——即 `import "github.com/linxlib/fw/v2"` 的那些仓库。

核心原则一句话：**把 fw 当外部依赖，扩展业务应用，不要改框架。** 只有当用户明确要求做框架开发时，才去动框架源码，那种场景用 `fw-development` skill。

本 skill 是可执行的步骤清单。**完整规范不在本文件里**，而在 fw 框架仓库的 markdown 文档中——见下一节，用之前先把它们拿到手。

## 第 1 步：拉取权威详版文档

**应用项目里没有框架文档。** 它们是 fw 框架仓库根目录下的 markdown，不在你的 `node_modules`、也不在 `go.sum` 里。本 skill 只是执行清单，遇到本文件没写清的规范，去读原文。

框架仓库：

- git 地址：`https://github.com/linxlib/fw`
- 默认分支：`v2`
- 模块路径：`github.com/linxlib/fw/v2`

需要的文件（仓库根目录）：

| 文件 | 内容 | 什么时候读 |
|---|---|---|
| `APP_AGENT.md` | 应用开发完整指南，**本 skill 的权威来源** | 动手前读一遍 |
| `doc_cn.md` | 中文完整框架文档（注解、绑定、中间件、DI、响应、OpenAPI、配置） | 查具体规则时 |
| `doc_en.md` | 英文完整框架文档，与 `doc_cn.md` 章节一一对应 | 同上，二选一 |
| `doc.md` | 框架概览与快速开始 | 了解全貌 |
| `AGENTS.md` | 框架架构与模块地图 | 需要理解框架内部时 |
| `config/README.md` | 配置库完整说明 | 写配置、加热重载时 |

四种获取方式，按当前环境选最快的：

**1. 开发机上已有 fw 源码检出（最常见，优先用）**

如果机器上已经 clone 了 fw 仓库，直接读那份根目录的 md，不用下载。不确定位置时：

```powershell
Get-ChildItem -Path 'E:/tmp','~/go/src/github.com/linxlib','~/code','~/projects' -Filter 'fw' -Directory -ErrorAction SilentlyContinue
```

**2. 读 Go 模块缓存（零网络，版本精确）**

依赖已经 download 过时，文档就在缓存里，按版本号分子目录：

```powershell
$gp = go env GOPATH
Get-ChildItem "$gp/pkg/mod/github.com/linxlib/fw" -Directory | Select-Object Name
# 取对应版本目录，例如 v2@v2.0.1/APP_AGENT.md
```

**3. 浅克隆（要检索多个文件时）**

```powershell
git clone --depth 1 -b v2 https://github.com/linxlib/fw $env:TEMP/fw-docs
```

**4. 只取单个 raw 文件（最轻量）**

```powershell
$base = "https://raw.githubusercontent.com/linxlib/fw/v2"
Invoke-WebRequest "$base/APP_AGENT.md" -OutFile "$env:TEMP/APP_AGENT.md"
```

### 版本必须对齐

文档和代码同一个 tag。先看应用依赖的版本，再取同版本文档，否则文档描述的行为可能和实际依赖不一致：

```powershell
Select-String -Path go.mod -Pattern 'linxlib/fw'
```

取到 `v2.x.y` 就把上面的 `v2` 换成 `v2.x.y`；用模块缓存或本地检出时，同样优先选匹配版本的目录。

如果拿不到网络也没有本地副本，**明确告诉用户"无法核对权威文档，以下按 skill 内的清单执行"**，不要凭空推断框架行为。

## 第 2 步：先摸清项目，别另起炉灶

1. 确认项目已有的目录结构。标准布局是 `controllers/` `services/` `middlewares/` `models/` `config/` `main.go`
2. **如果项目已经是另一套清晰的结构，遵循现有结构**，不要强行改成标准布局
3. 读一遍 `main.go`，看清启动方式（是否 embed `.astp.json`、是否用了 `EnableConfigReload`）
4. 找到与任务相关的 controller / service / model，先读再改

## 新增一个 API 的标准流程

按这个顺序做，不要跳步：

1. 在 `models/` 加或改请求/响应结构
2. 在 `services/` 加或改业务逻辑
3. 在 `controllers/` 加处理方法
4. 写路由注解
5. 需要横切行为时加中间件注解
6. **重新生成元数据**（见下文，这步最常被漏）

示例：

```go
package controllers

import (
	"your/module/models"
	"your/module/services"

	"github.com/linxlib/fw/v2/context"
	"github.com/valyala/fasthttp"
)

// UserController handles user APIs.
// @Controller
// @Route /api/v1/users
type UserController struct {
	Service *services.UserService
}

// Create creates a user.
// @POST /
func (c *UserController) Create(ctx context.Context, body models.CreateUserBody) error {
	result := c.Service.Create(body)
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", result)
}
```

## 分层职责

| 目录 | 放什么 | 不放什么 |
|---|---|---|
| `controllers/` | HTTP 入口、路由注解、参数装配、传输层响应 | 业务规则、大段工作流 |
| `services/` | 业务规则，可复用方法 | HTTP 细节、`ctx`、状态码 |
| `middlewares/` | 认证、鉴权、审计、追踪、限流等横切行为 | **业务逻辑** |
| `models/` | 请求/响应结构 | 行为方法 |
| `config/app.yaml` | 环境相关行为 | 硬编码的魔法数 |

拿不准时：能在应用层干净解决就在应用层解决。

## Controller 规范

- struct 上标 `@Controller`
- 用 `@Route /base` 指定基路径，支持 `@Route /base` 和 `@Route(/base)` 两种写法
- 方法上用 `@GET` `@POST` `@PUT` `@DELETE` `@PATCH` `@OPTIONS` `@HEAD` 定义路由
- **保持 controller 薄**，业务逻辑下沉到 service
- 同一方法可以写多条同方法注解，只要 URL 不同：

```go
// @POST /user
// @POST /user_info
func (c *UserController) ModifyUser(ctx context.Context, body models.ModifyUserBody) error {
	return nil
}
```

注册成两条路由。重复的 `METHOD + PATH` 会被拒绝。

### 泛型基类

把增删改查沉淀成泛型基类，业务 controller 内嵌实例化后的类型即可：

```go
// BaseController 是泛型增删改查基类。
type BaseController[T any] struct{}

// @GET /
func (c *BaseController[T]) List(ctx context.Context, query models.PageQuery) (models.PageSize[*T], error) {
	return models.PageSize[*T]{}, nil
}

// @POST /
func (c *BaseController[T]) Create(ctx context.Context, entity *T) error {
	return nil
}

// UserController 处理用户接口。
// @Controller
// @Route /api/v1/users
type UserController struct {
	BaseController[models.User]
}
```

方法会被提升到外层 controller，OpenAPI 的 schema 会按实参展开成真实实体字段。注意：**内嵌提升仅限同包**，跨包内嵌暂不提升。

## Service 规范

- 标 `@Service` 才会被自动注册生成器发现
- 优先写 `NewXxxService()` 构造函数且零参数；生成器找不到构造函数时退回注册 `&XxxService{}`
- 不想被自动注册时加 `@Disable`
- service 方法不碰 HTTP：不传 `ctx`、不返回状态码

```go
package services

import "your/module/models"

// OrderService contains order logic.
// @Service
type OrderService struct{}

func NewOrderService() *OrderService {
	return &OrderService{}
}

func (s *OrderService) Create(body models.CreateOrderBody) map[string]any {
	return map[string]any{"id": body.ID}
}
```

## Model 与参数绑定

### 命名即推断

模型名后缀就是绑定来源提示，这是最省事的做法：

- `XxxBody` → 请求体
- `XxxQuery` → query
- `XxxPath` → 路径参数
- `XxxHeader` → 请求头

必要时用类型注解显式指定：`@Body` / `@Query` / `@Path` / `@Header`。

```go
// CreateOrderBody request payload.
// @Body
type CreateOrderBody struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ListOrderQuery query parameters.
// @Query
type ListOrderQuery struct {
	Page     int    `query:"page"`
	PageSize int    `query:"page_size"`
	Keyword  string `query:"keyword"`
}
```

可用 tag：`json` `query` `path` `header`，以及给 OpenAPI 用的 `example` `default`。

### 推断顺序

每个处理函数参数按这个顺序判定来源：

1. 参数类型带 `@Query` / `@Path` / `@Header` / `@Body` → 用注解指定的
2. 类型名以 `Query` / `Path` / `Header` / `Body` 结尾 → 作为提示
3. 参数名匹配路由占位符（如 `:id`）→ 路径参数
4. 解析出的结构体类型 → 默认请求体
5. 其他基础类型 → 默认 query

### 容易踩的绑定规则

- query、header、body 字段缺失时**保留零值**，可选的筛选条件不必写成指针
- **路径参数对不上会直接报错**：路由声明了 `:id` 但处理方法没写 `id` 形参（或反过来）不会静默绑空值
- 每个参数独立绑定，两个相同基础类型的参数不会共享同一个解析结果
- 裸的基础类型参数（如 `page int`）没有 Tag，无法标注 `example`/`default`；想给文档写默认值就收敛成 struct

### 让 OpenAPI 有真实示例值

模型字段加 `example` 与 `default` tag，生成的 schema 才会带真实值，否则只有按类型给的占位值：

```go
type PageQuery struct {
	Page int `json:"page" query:"page" default:"1" example:"1"`
	Size int `json:"size" query:"size" default:"10" example:"10"`
}
```

## 中间件

只用于横切行为。适用场景：认证、鉴权、审计日志、请求追踪、租户解析、限流、SSE/WebSocket 协议适配。

```go
// AuthMiddleware checks API keys.
// @Middleware
// @Global
type AuthMiddleware struct{}
```

- `@Global` + `@Middleware`：生成代码调用 `e.Use(...)`，对所有路由生效
- 只有 `@Middleware`：生成代码调用 `e.RegisterMiddleware(...)`，仅在被注解引用时生效
- 控制器级 / 方法级注解按名字匹配；方法级覆盖控制器级同名中间件
- 同一中间件在多层绑定时**只执行一次**，保留最具体的一层
- 跳过某个路由的中间件：

```go
// @GET /health
// @Ignore(Auth,Global)
```

`@Ignore(Auth)` 忽略指定中间件，`@Ignore(Global)` 忽略全部全局中间件，多条 `@Ignore` 会合并。

### 中间件读配置

中间件配置来自 `config/app.yaml` 的 `middlewares:` section，按中间件名小写匹配。没有对应段时得到空 `config.Section`，**无需判空**：

```go
type AuthorizationMiddleware struct {
	Config *config.Section `inject:""`
}

func (m *AuthorizationMiddleware) Handle(ctx context.Context, _ middleware.AnnotationArgs, next middleware.Handler) error {
	if m.Config.Get("api-key") == "" {
		return ctx.Respond(fasthttp.StatusUnauthorized, 40100, "missing api key", nil)
	}
	return next(ctx)
}
```

`config.Section` 是 `map[string]any`，key 大小写不敏感，只有 `Get` 和 `Has`。环境变量进不了 map，所以中间件配置只能写在 YAML 里。

## 依赖注入

两种方式，按依赖性质选：

```go
// 主依赖用结构体字段
type UserController struct {
	Service *services.UserService
}

// 请求级的、辅助的依赖用参数注入
func (c *UserController) Create(ctx context.Context, body models.CreateUserBody, svc *services.UserService) error {
	result := svc.Create(body)
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", result)
}
```

也可以手动映射：`e.Container().Map(services.NewUserService())`。

## 响应规范

默认走统一外壳，不要手写 JSON：

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "trace_id": "..."
}
```

标准调用：

```go
return ctx.Respond(fasthttp.StatusOK, 0, "ok", data)
```

签名是 `Respond(statusCode int, code int, message string, data any) error`。

- 相似接口的 code / message 保持一致
- controller 负责整形传输层响应；service 返回领域数据，**不要**在 service 里构造 HTTP 外壳
- 处理方法返回 `(数据, error)` 时框架自动包外壳；返回非 error 的第一个值作为 `data`
- 需要输出原始 JSON 时用 `@RawResponse` 或 `ctx.RawResponse()`，但要有理由，别默认绕开外壳

## 配置

配置文件是 `config/app.yaml`。section 通过 `inject` tag 映射到结构体字段，`default` tag 提供缺省值，所以 YAML 里缺省的段仍有 sane default。

```yaml
server:
  host: "0.0.0.0"
  port: 8080

log:
  level: info
  output: console
  request_enabled: true

openapi:
  enabled: true
  output: openapi.json
  title: "fw API"
  version: "1.0.0"

recovery:
  enabled: true
  return_stack_to_body: false

middlewares:
  authorization:
    api-key: xxxx
```

环境变量按 `FW_<SECTION>_<FIELD>` 派生，显式 `env:"NAME"` tag 优先：

```bash
FW_SERVER_PORT=9090
FW_LOG_LEVEL=debug
```

新增配置驱动的行为时：加 YAML 条目 → 在应用里写清默认值 → 保证环境变量名可预测。

## 元数据重新生成（最关键的一步）

项目通过 `.astp.json` 嵌入 AST 元数据。**只要改了带注解的类型，就必须重新生成**，否则 `go run .` 跑的还是旧元数据，表现为"代码改了但路由没变"。

需要重新生成的情况：

- 增加 / 改名 `@Controller` `@Service` `@Middleware` 类型
- 改路由注解
- 改控制器或中间件名
- 改变项目结构以致影响 AST 扫描

命令：

```powershell
fw build      # 重新生成 .astp.json 与 fw_autoreg_gen.go 并构建
go generate ./...   # 至少做到这一步
```

`go:generate` 那一行通常在 `main.go` 里：

```go
//go:generate go run github.com/linxlib/fw/v2/astp/cmd/astp -exported .
//go:embed .astp.json
var astpData []byte
```

## 配置热重载（可选）

默认关闭。需要时在 `app.New` 之后、`ListenAndServe` 之前调用：

```go
e.EnableConfigReload(time.Second)
```

行为：按顶层 section 增量重载，标准输出打印 `config: reload detected, changed sections: [...]`，未变化时不打印。

**限制要记牢**：路由表和监听地址在启动时已确定，重载不会重建路由、不会切换端口。所以改 `server.port` 后必须重启进程。

## 交付前检查清单

逐条确认，不要跳：

- [ ] 路由能编译通过
- [ ] 改了带注解的类型 → 已跑 `fw build` 或 `go generate ./...`
- [ ] `go test ./...` 通过
- [ ] 新接口用了正确的注解，`@Service` / `@Middleware` 标在了该标的类型上
- [ ] 没有重复路由（同 `METHOD + PATH` 会被拒绝）
- [ ] 业务逻辑在 service 里，不在 middleware 或 controller 里
- [ ] 走了标准响应外壳，没有无理由绕开
- [ ] 模型命名能表达绑定来源，字段带了 `example` / `default`
- [ ] 配置改动已写进 `config/app.yaml`

推荐命令：

```powershell
fw build
go test ./...
```

## 常见错误

- 忘了跑 `fw build`，改了路由却不生效
- 把业务逻辑塞进中间件或控制器
- 在应用项目里顺手改框架源码
- 模型名含糊（`Data`、`Params`），导致绑定来源难判断
- 以为路径参数缺失会给零值——实际会直接报错
- service 里返回 HTTP 外壳，把传输层关注点漏进业务层
- 以为改 `server.port` 能热切换监听端口
