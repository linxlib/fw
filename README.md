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

## Install

Add FW to your project:

```bash
go get github.com/linxlib/fw/v2
```

Install the CLI if needed:

```bash
go install github.com/linxlib/fw/v2/cmd/fw@latest
```

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
