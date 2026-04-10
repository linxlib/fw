# FW Documentation

FW is a lightweight Go API framework built on `fasthttp`, centered around annotation-driven routing, middleware binding, dependency injection, and automatic OpenAPI generation.

Full guides:

- Chinese: [doc_cn.md](./doc_cn.md)
- English: [doc_en.md](./doc_en.md)

## At a Glance

- Annotation-based route definition such as `@GET /users` and controller base route `@Route /api/v1`
- Automatic discovery and registration of `@Controller`, `@Service`, and `@Middleware`
- Global, controller-level, and method-level middleware with deterministic merge order
- Request parameter injection from path, query, header, and JSON body
- Built-in request context wrapper, automatic method return responses, unified response envelope, and panic recovery
- Automatic OpenAPI generation plus Swagger UI at `/docs`
- `cmd/fw` CLI for project scaffolding, building, and component generation

## Quick Start

Create a new project:

```bash
fw init demo
cd demo
```

Generate metadata and build the project:

```bash
fw build
```

Run the app during development:

```bash
go run .
```

Note: the scaffolded project embeds `.astp.json`, so you should run `fw build` or `go generate ./...` at least once before `go run .`.

## Core Usage

Define a controller:

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

// GetUser returns one user.
// @GET /:id
func (c *UserController) GetUser(ctx context.Context, id string) error {
	return ctx.StatusOK().Data(map[string]any{
		"id": id,
	})
}

// GetProfile returns raw JSON without the default envelope.
// @GET /profile
// @RawResponse
func (c *UserController) GetProfile() map[string]any {
	return map[string]any{"name": "alice"}
}
```

Define a service:

```go
package services

// UserService contains user business logic.
// @Service
type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}
```

Define a middleware:

```go
package middlewares

import (
	"github.com/linxlib/fw/v2/context"
	"github.com/linxlib/fw/v2/middleware"
)

// AuthMiddleware checks authentication.
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

func (AuthMiddleware) Handle(ctx context.Context, _ middleware.AnnotationArgs, next middleware.Handler) error {
	return next(ctx)
}
```

## `cmd/fw` CLI

Initialize a project:

```bash
fw init myapp
```

Build the current project:

```bash
fw build
fw build -o myapp
fw build -os linux -arch amd64
```

Generate components:

```bash
fw create controller --name User --route /api/v1/users
fw create service --name User
fw create middleware --name Authorization
```

Other commands:

```bash
fw version
fw completion bash
```

## Recommended Reading

- Read [doc_cn.md](./doc_cn.md) if you want the full Chinese guide.
- Read [doc_en.md](./doc_en.md) if you want the full English guide.
