# FW Agent Guide

This document describes the current project architecture and conventions for future development.

## Module

- Module path: `github.com/linxlib/fw/v2`
- Go version: `1.25`
- Runtime stack:
  - `github.com/valyala/fasthttp`
  - `github.com/fasthttp/router`
  - `github.com/pterm/pterm`
  - `gopkg.in/yaml.v3`

## Directory Layout

- `app/`: engine lifecycle, controller registration, route build and runtime dispatch
- `annotation/`: normalized annotation extraction from `astp` output
- `astp/`: in-repo AST parser library (enhanced annotation parsing)
- `inject/`: in-repo dependency injection container (enhanced invoke resolver pipeline)
- `config/`: yaml + env override configuration loader
- `logger/`: colored console + plain file logger
- `context/`: request context wrapper around `*fasthttp.RequestCtx`
- `middleware/`: middleware protocol and chain execution model
- `response/`: default response envelope + pluggable formatter/encoder
- `openapi/`: OpenAPI JSON generation
- `cmd/example/`: runnable minimal example

## Core Runtime Model

### Engine

Main type: `app.Engine`

Key responsibilities:

1. Load config
2. Parse project metadata with `astp`
3. Register controllers and annotations to routes
4. Resolve middleware bindings
5. Register `fasthttp` handlers
6. Print route tree and generate `openapi.json`

## Annotation Semantics

### Built-in route annotations

- Supported HTTP annotations: `@GET`, `@POST`, `@PUT`, `@DELETE`, `@PATCH`, `@OPTIONS`, `@HEAD`
- Controller base path: `@Route /base` or `@Route(/base)`
- One method can define multiple same-method route annotations as long as URLs differ.

Example:

```go
// @POST /api/v1/user
// @POST /api/v1/user_info
func (c *Controller) ModifyUser(...) {}
```

Registers two routes.

### Middleware annotations

- Middleware is discovered by annotation name from registered middleware specs.
- Same location (controller or method): same annotation name uses **last one wins**.
- Across levels: method-level annotation overrides controller-level annotation when names are same.
- Different middleware names are merged.

Execution order:

1. global before
2. controller before
3. method before
4. handler
5. method after
6. controller after
7. global after

## ASTP Enhancements

`astp.CommentGroup` now includes:

- `Annotations []string` (compat)
- `ParsedAnnotations []*Annotation` (structured)

`astp.Annotation`:

- `Name`
- `Raw`
- `Args []string`
- `KV map[string]string`

Supported annotation styles:

- `@X`
- `@X value`
- `@X(a,b)`
- `@X(k=v, role=admin)`

## Inject Enhancements

`inject.Injector` now supports:

- `InvokeWith(func, names, source)`
- custom resolver registration via `RegisterResolver`

Framework usage:

- Global injector: app-wide services
- Request injector: per-request container with parent = global
- Request injector maps:
  - `context.Context`
  - `*context.FWContext`
  - `*fasthttp.RequestCtx`
- Resolver can bind primitive args and struct/pointer body/query/path models.

## Response Model

Default envelope:

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "trace_id": "..."
}
```

Extensible via:

- `response.Formatter`
- `response.Encoder`

Set by `Engine.SetResponse(...)`.

## OpenAPI

- Generated at startup when enabled
- Output file defaults to `openapi.json`
- Current generation includes path/method/operation id/basic 200 response

## Config

Load order:

1. Defaults
2. YAML file (if provided)
3. Env override (prefix `FW_`)

Example env keys:

- `FW_SERVER_HOST`
- `FW_SERVER_PORT`
- `FW_LOG_LEVEL`
- `FW_OPENAPI_ENABLED`

## Current Example

`cmd/example/main.go` demonstrates:

- controller-level `@Route` and `@Authorization(Admin)`
- method-level duplicate `@POST`
- method-level `@Log(stage=before)`
- middleware registration and startup

## Development Notes

- Keep `astp` and `inject` as internal subpackages and evolve in-repo.
- Prefer backward-compatible API changes when possible.
- Keep framework-level behavior deterministic (especially annotation merge and middleware order).
- Run `go test ./...` before delivering changes.
