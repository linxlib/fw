# FW App Development Guide For Coding Agents

This document is for coding agents building business applications that use FW as a Go module dependency.

Use this guide when the application imports:

```go
github.com/linxlib/fw/v2
```

This is not a guide for changing the FW framework repository itself.

## Goal

Build application features on top of FW in a clean and predictable way:

- add business endpoints
- organize code into controllers, services, middlewares, and models
- use FW annotations and dependency injection correctly
- keep generated metadata up to date
- avoid treating the app project like the FW framework source repo

## Core Principle

When working in an FW-based business project, focus on the application code in that project.

Assume FW is an external dependency used through Go modules, not something you should modify during normal feature work.

Only change FW itself if the user explicitly asks for framework development.

## Recommended Application Layout

A typical FW application should organize code like this:

- `controllers/`: HTTP handlers and route annotations
- `services/`: business logic
- `middlewares/`: cross-cutting request behavior
- `models/`: request and response models
- `config/`: YAML config files
- `main.go`: application startup

If the project already uses a different but clear structure, follow the existing layout.

## Typical Development Flow

For most feature work, use this order:

1. Understand the requested API or behavior.
2. Find the relevant controller, service, middleware, and models.
3. Add or update models.
4. Add or update service logic.
5. Add or update controller methods and route annotations.
6. Add middleware only if cross-cutting behavior is needed.
7. Add config only if the feature really needs configuration.
8. Regenerate FW metadata if annotated types changed.
9. Run tests.

## Module Usage

An FW application usually depends on the framework like this:

```go
import "github.com/linxlib/fw/v2/app"
```

The project may also import:

- `github.com/linxlib/fw/v2/context`
- `github.com/linxlib/fw/v2/middleware`
- `github.com/valyala/fasthttp`

Treat FW as the foundation of the app, not as local app code.

## Startup Pattern

A typical FW application startup looks like this:

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

If the app wants live config changes, enable hot reload between `app.New` and `ListenAndServe`:

```go
e.EnableConfigReload(time.Second)
```

It reloads only the top-level sections whose content changed and prints
`config: reload detected, changed sections: [...]` on standard output. Routes and the
listening address are fixed at startup, so a changed `server.port` does not move the listener.

Important:

- if the project embeds `.astp.json`, keep it up to date
- after adding or renaming controllers, services, or middlewares, run `fw build` or at least `go generate ./...`

## How To Add A New API

The most common task is adding a new endpoint.

Use this pattern:

1. Add or update request and response models in `models/`
2. Add or update business logic in `services/`
3. Add a controller method in `controllers/`
4. Add route annotations
5. Use middleware annotations if needed
6. Rebuild generated metadata

Example:

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

## Controllers

Controllers are the HTTP entrypoints.

Rules:

- mark controller structs with `@Controller`
- use `@Route` for the base path
- define routes on methods with `@GET`, `@POST`, `@PUT`, `@DELETE`, `@PATCH`, `@OPTIONS`, or `@HEAD`
- keep controllers thin
- put business logic into services

Example:

```go
// OrderController handles order APIs.
// @Controller
// @Route /api/v1/orders
type OrderController struct {
	Service *services.OrderService
}

// Detail gets one order.
// @GET /:id
func (c *OrderController) Detail(ctx context.Context, id int64) error {
	data := c.Service.Detail(id)
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", data)
}
```

## Services

Services hold business logic.

Mark them with `@Service` so FW auto-registration can discover them.

Example:

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
	return map[string]any{
		"id": body.ID,
	}
}
```

Service guidance:

- keep HTTP details out of services
- keep service methods reusable
- prefer constructor functions like `NewOrderService()`

## Models

Put request and response shapes in `models/`.

FW can bind structs from body, query, path, and headers.

Example body model:

```go
package models

// CreateOrderBody request payload.
// @Body
type CreateOrderBody struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}
```

Example query model:

```go
package models

// ListOrderQuery query parameters.
// @Query
type ListOrderQuery struct {
	Page     int    `query:"page"`
	PageSize int    `query:"page_size"`
	Keyword  string `query:"keyword"`
}
```

Model guidance:

- keep transport models in `models/`
- use tags like `json`, `query`, `path`, and `header`
- use clear names such as `CreateUserBody`, `ListUserQuery`, `UserPath`

## Parameter Binding

FW supports automatic binding for handler parameters.

Common patterns:

- path primitives
- query primitives
- body structs
- query structs
- injected services

Example:

```go
// @GET /:id
func (c *UserController) Detail(ctx context.Context, id int64, query models.DetailUserQuery) error {
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", map[string]any{
		"id":    id,
		"query": query,
	})
}
```

Use explicit model naming to help the framework infer binding source:

- `SomethingBody`
- `SomethingQuery`
- `SomethingPath`
- `SomethingHeader`

If needed, add type annotations such as `@Body` or `@Query`.

Notes on binding:

- a missing query, header, or body field keeps the zero value, so optional filters do not need pointer types
- each parameter is bound on its own, so two parameters of the same primitive type never share a value

## Middleware

Use middlewares for cross-cutting behavior, not business logic.

Good middleware use cases:

- authentication
- authorization
- audit logging
- request tracing
- tenant resolution
- rate limiting
- protocol adapters like SSE and WebSocket

Example:

```go
// AuthMiddleware checks API keys.
// @Middleware
// @Global
type AuthMiddleware struct{}
```

Middleware usage tips:

- global middleware for app-wide behavior
- controller-level middleware for feature area rules
- method-level middleware for endpoint-specific behavior

Skip middleware on a route when needed:

```go
// @Ignore(Auth,Global)
```

## Dependency Injection

FW supports:

- service injection into controllers
- service injection into handler parameters
- request-scoped injection

Common pattern:

```go
type UserController struct {
	Service *services.UserService
}
```

Handler example:

```go
func (c *UserController) Create(ctx context.Context, body models.CreateUserBody, svc *services.UserService) error {
	result := svc.Create(body)
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", result)
}
```

Recommendation:

- prefer controller fields for the main service dependency
- use parameter injection for request-scoped or auxiliary dependencies

## Responses

Use the framework response style instead of writing raw JSON manually unless the task requires custom behavior.

Standard response call:

```go
return ctx.Respond(fasthttp.StatusOK, 0, "ok", data)
```

Default envelope:

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "trace_id": "..."
}
```

Recommendation:

- keep response codes and messages consistent across similar endpoints
- let controllers shape transport responses
- let services return domain data, not HTTP-specific envelopes

## Config

Application config usually lives in `config/app.yaml`.

Config sections map to struct fields through `inject` tags, and `default` tags hold the
fallback value, so a section that is absent from the YAML file still has a sane default.
Environment overrides are derived as `FW_<SECTION>_<FIELD>`, and an explicit `env:"NAME"` tag
wins over the derived name.

Typical examples:

- server host and port
- logging
- OpenAPI settings
- middleware settings
- application-specific feature config

If you add config-driven behavior:

1. add the YAML config entry
2. document the intended default in the app if needed
3. ensure environment override names remain predictable

## `cmd/fw` CLI In App Development

The CLI is useful during normal app development.

### Initialize a new project

```bash
fw init myapp
```

### Build and regenerate metadata

```bash
fw build
```

Use this after:

- adding controllers
- adding services
- adding middlewares
- renaming annotated types
- changing project structure affecting AST scanning

### Generate boilerplate

```bash
fw create controller --name User --route /api/v1/users
fw create service --name User
fw create middleware --name Authorization
```

## Regeneration Rule

If your task changes any of these:

- `@Controller` types
- `@Service` types
- `@Middleware` types
- route annotations
- controller or middleware names

then you should usually run:

```bash
fw build
```

Why:

- it regenerates `.astp.json`
- it regenerates `fw_autoreg_gen.go`
- it catches registration and build issues early

## OpenAPI Awareness

FW generates OpenAPI automatically.

When adding endpoints, think about:

- request model shape
- response model shape
- path params
- query params
- security middleware

If an endpoint returns a meaningful payload, make sure the handler signature helps OpenAPI infer the schema clearly.

For models, annotate fields with `example:"..."` and `default:"..."` so the generated schema
carries real values instead of type placeholders. Generic base controllers work too: embed an
instantiated base such as `BaseController[models.User]`, and the promoted methods get real
schemas for `T`.

## SSE And WebSocket

Use these only when the task specifically needs streaming or full-duplex communication.

### SSE pattern

```go
// @GET /stream
// @SSE
func (c *StreamController) Events(ctx context.Context, w *context.SSEWriter) error {
	return nil
}
```

### WebSocket pattern

```go
// @GET /ws
// @WS
func (c *SocketController) Echo(ctx context.Context, msg []byte) error {
	ctx.Set("WS.Reply", msg)
	return nil
}
```

Do not choose SSE or WebSocket unless the requirement actually calls for them.

## What Agents Should Avoid

Avoid these mistakes:

- treating FW like local framework source that should be edited during normal app work
- putting business logic into middleware
- putting large business workflows directly into controllers
- forgetting to run `fw build` after adding annotated types
- writing handlers that bypass FW response conventions without a reason
- creating duplicate routes
- using unclear model names that make binding harder to understand

## Verification Checklist

Before finishing an app-development task, check:

- routes compile
- generated metadata is up to date if needed
- `go test ./...` passes
- config changes are reflected in `config/app.yaml` if applicable
- new endpoints use the right annotations
- services are marked with `@Service` if they should be auto-registered
- middlewares are marked correctly if they should be auto-registered

Recommended commands:

```bash
fw build
go test ./...
```

## Decision Guide

When implementing a feature, choose the layer like this:

- use `controllers/` for HTTP endpoints
- use `services/` for business rules
- use `middlewares/` for cross-cutting request behavior
- use `models/` for request and response structures
- use `config/` for environment-dependent behavior

If you can solve the task cleanly in the application layer, do that.

## Summary

For coding agents building on FW:

- treat FW as an external application foundation used through Go modules
- extend the business application, not the framework
- use annotations, DI, and standard response flow consistently
- regenerate metadata after annotated type changes
- verify with `fw build` and `go test ./...`
