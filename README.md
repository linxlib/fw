# FW

[中文说明 / Chinese README](./README_CN.md)

FW is a lightweight Go web framework for building APIs quickly with annotation-style routing, middleware binding, dependency injection, and automatic OpenAPI generation.

## Features

- Annotation-based routes such as `@GET /users`
- Auto-registration for controllers, services, and middlewares
- Global, controller-level, and method-level middleware
- Automatic request binding for path, query, header, and body
- Unified JSON response envelope
- OpenAPI generation with Swagger UI
- `cmd/fw` CLI for scaffolding and builds
- Optional config hot reload, applied per top-level section
- Generic controllers and methods, with real OpenAPI schemas inferred from type arguments

## Install

Add FW to your project:

```bash
go get github.com/linxlib/fw/v2
```

Install the CLI if needed:

```bash
go install github.com/linxlib/fw/v2/cmd/fw@latest
```

**Requirements:** Go 1.27 or later. Starting with **v2.1.0** the module declares
`go 1.27` in `go.mod`, so older toolchains refuse to build it (`go.mod requires go >= 1.27`).
Go 1.27 is needed to parse Go 1.27 method generics.

## Upgrading from v2.0.x and earlier

v2.1.0 keeps the module path `github.com/linxlib/fw/v2`, so upgrading is a plain
`go get` with no import rewrite — but it is **not source-compatible** if you use the
`config` package directly.

### Code changes required

| v2.0.1 | v2.1.0 |
| --- | --- |
| `config.Load(path string) (Config, error)` | `config.Load(target any) error` |
| `config.Default()` | removed — use `app.DefaultEngineConfig()` |
| `config.ServerConfig` / `LogConfig` / `RecoveryConfig` / `OpenAPIConfig` | moved to the `app` package |
| `Config.Server` / `.Log` / `.OpenAPI` / `.Recovery` / `.ProjectDir` / `.Middlewares` | removed — `Config` is now a loader built with `config.New(&config.Option{...})` |
| `Section` struct with `GetString` / `GetInt` / `GetBool` / `GetStrings` / `GetDuration` / `Sub` / `MustGet` / `UnmarshalYAML` | `Section` is `map[string]any`; only `Get` and `Has` remain |

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
_ = c.LoadWithKey("server", &o) // or c.Load(&o) / c.LoadByTags(&o)

// middleware config: Section is a plain map, assert values yourself
if mw.Config.Has("timeout") {
	d, _ := time.ParseDuration(mw.Config.Get("timeout"))
}
```

If you only call `app.New("config/app.yaml")`, no code change is needed.

### `openapi` package

`RouteInfo`, `MediaType` and `Schema` gained fields (`ResponseExample`, `Example`,
`Default`). Positional composite literals without field names no longer compile — add
field names.

### Behaviour that changes without a compile error

- **Environment variables are derived differently.** Old: from the `yaml` tag
  (`FW_LOG_FILE_PATH`, `FW_RECOVERY_RETURN_STACK_TO_BODY`). New: from the `inject` key
  plus the Go field name (`FW_LOG_FILEPATH`, `FW_RECOVERY_RETURNSTACKTOBODY`).
  `FW_SERVER_HOST`, `FW_SERVER_PORT`, `FW_LOG_LEVEL` and `FW_OPENAPI_ENABLED` are
  unchanged. `FW_MIDDLEWARES_*` no longer works — middleware config is YAML-only.
- **A missing config file no longer fails startup**; defaults are used instead. Only
  `.yaml` files are accepted.
- **Defaults changed:** `log.file_path` `logs/fw.log` → `""`,
  `recovery.return_stack_to_body` `true` → `false`,
  `openapi.title` `FW API` → `fw API`.
- **The same middleware bound at several levels now runs once**, keeping the most
  specific binding (method > controller > global). Deduplication is by *name*, so two
  different implementations sharing a name means the less specific one never runs.
- **Collection routes lose the trailing slash:** `@Route /users` + `@GET /` now maps to
  `/users`. Clients hard-coding `/users/` get 404; `/users` answers directly instead of
  301-redirecting.
- **Regenerate `.astp.json`** (`go generate ./...`) if you deploy with embedded AST
  metadata; v2.0.1 files miss generic base-class routes.

### Fixes worth knowing

- Missing query/header parameters fall back to the zero value instead of returning 500.
- Same-type parameters no longer leak each other's values (`?page=2&size=5`).
- `:name` path parameters are actually reachable — on v2.0.1 they silently 404'd
  because fasthttp/router only understands `{name}`.

## Quick Start

Create a new project:

```bash
fw init demo
cd demo
fw build
go run .
```

Typical startup:

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

Simple controller:

```go
package controllers

import (
	"github.com/linxlib/fw/v2/context"
	"github.com/valyala/fasthttp"
)

// UserController handles user APIs.
// @Controller
// @Route /api/v1/users
type UserController struct{}

// Detail returns one user.
// @GET /:id
func (c *UserController) Detail(ctx context.Context, id string) error {
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", map[string]any{"id": id})
}
```

## Documentation

- Overview docs: [doc.md](./doc.md)
- Full English guide: [doc_en.md](./doc_en.md)
- Full Chinese guide: [doc_cn.md](./doc_cn.md)

For coding agents:

- App development guide: [APP_AGENT.md](./APP_AGENT.md)
- Documentation maintenance guide: [DOCS_AGENT.md](./DOCS_AGENT.md)

## CLI

Useful commands:

```bash
fw init myapp
fw build
fw create controller --name User --route /api/v1/users
fw create service --name User
fw create middleware --name Authorization
```

See the full CLI details in [doc_en.md](./doc_en.md) and [doc_cn.md](./doc_cn.md).

## License

[MIT](./LICENSE)
