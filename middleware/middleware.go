package middleware

import (
	"github.com/linxlib/fw/context"
)

type Handler func(context.Context) error

// Middleware is the core middleware interface using the onion (Handle + next) model.
// Implementations receive the request context, annotation arguments, and a next function
// that invokes the rest of the middleware chain plus the final handler.
//
// Simple before/after logic:
//
//	func (m *MyMW) Handle(ctx context.Context, args AnnotationArgs, next Handler) error {
//	    // before
//	    if err := next(ctx); err != nil { return err }
//	    // after
//	    return nil
//	}
//
// Hijacking (e.g. WebSocket): the middleware may call next() zero or many times,
// and may block in a loop.
type Middleware interface {
	Spec() AnnotationSpec
	Handle(ctx context.Context, args AnnotationArgs, next Handler) error
}

// BeforeAfter is an optional backward-compatible interface. If a middleware only needs
// simple Before/After hooks, it can implement this and be wrapped via WrapBeforeAfter.
type BeforeAfter interface {
	Before(ctx context.Context, args AnnotationArgs) error
	After(ctx context.Context, args AnnotationArgs) error
}

// WrapBeforeAfter adapts a BeforeAfter into a full Middleware.
func WrapBeforeAfter(spec AnnotationSpec, ba BeforeAfter) Middleware {
	return &beforeAfterAdapter{spec: spec, ba: ba}
}

type beforeAfterAdapter struct {
	spec AnnotationSpec
	ba   BeforeAfter
}

func (a *beforeAfterAdapter) Spec() AnnotationSpec { return a.spec }

func (a *beforeAfterAdapter) Handle(ctx context.Context, args AnnotationArgs, next Handler) error {
	stage := a.spec.Stage
	if stage == StageBefore || stage == StageBoth {
		if err := a.ba.Before(ctx, args); err != nil {
			return err
		}
	}
	if err := next(ctx); err != nil {
		return err
	}
	if stage == StageAfter || stage == StageBoth {
		if err := a.ba.After(ctx, args); err != nil {
			return err
		}
	}
	return nil
}

type Scope string

const (
	ScopeController Scope = "controller"
	ScopeMethod     Scope = "method"
	ScopeBoth       Scope = "both"
)

type Stage string

const (
	StageBefore Stage = "before"
	StageAfter  Stage = "after"
	StageBoth   Stage = "both"
)

type AnnotationSpec struct {
	Name  string
	Scope Scope
	Stage Stage
}

type AnnotationArgs struct {
	Args []string
	KV   map[string]string
}

// SecuritySchemeType defines supported OpenAPI security scheme types.
type SecuritySchemeType string

const (
	SecurityHTTP   SecuritySchemeType = "http"
	SecurityAPIKey SecuritySchemeType = "apiKey"
)

// SecuritySchemeIn defines where an apiKey is sent.
type SecuritySchemeIn string

const (
	SecurityInHeader SecuritySchemeIn = "header"
	SecurityInQuery  SecuritySchemeIn = "query"
	SecurityInCookie SecuritySchemeIn = "cookie"
)

// SecurityScheme describes an OpenAPI security scheme that an auth middleware provides.
type SecurityScheme struct {
	// Name is the security scheme identifier (e.g. "BasicAuth", "BearerAuth", "ApiKeyAuth").
	Name string
	// Type is the scheme type: "http" or "apiKey".
	Type SecuritySchemeType
	// Scheme is the HTTP auth scheme, used when Type is "http" (e.g. "basic", "bearer").
	Scheme string
	// BearerFormat is optional, e.g. "JWT", used when Scheme is "bearer".
	BearerFormat string
	// In is where the apiKey is sent, used when Type is "apiKey" (e.g. "header", "query", "cookie").
	In SecuritySchemeIn
	// FieldName is the header/query/cookie name, used when Type is "apiKey" (e.g. "Authorization", "X-API-Key").
	FieldName string
	// Description is an optional description for the security scheme.
	Description string
}

// SecurityProvider is an optional interface that auth middlewares can implement
// to declare their OpenAPI security scheme. When a middleware implements this interface,
// the framework will automatically add the corresponding security scheme to the
// OpenAPI document and mark operations protected by this middleware with a lock icon.
type SecurityProvider interface {
	SecurityScheme() SecurityScheme
}

type Bound struct {
	MW   Middleware
	Args AnnotationArgs
}

// Chain builds a Handler that executes middleware layers in onion order:
// globalBefore → controllerBefore → methodMiddleware → handler → (unwinding)
//
// Each middleware's Handle receives a next function that calls the next layer.
func Chain(handler Handler, globalMW []Bound, controllerMW []Bound, methodMW []Bound) Handler {
	// Flatten into a single ordered slice: global → controller → method
	all := make([]Bound, 0, len(globalMW)+len(controllerMW)+len(methodMW))
	all = append(all, globalMW...)
	all = append(all, controllerMW...)
	all = append(all, methodMW...)

	// Build the chain from inside out
	h := handler
	for i := len(all) - 1; i >= 0; i-- {
		bound := all[i]
		inner := h // capture
		h = func(ctx context.Context) error {
			return bound.MW.Handle(ctx, bound.Args, inner)
		}
	}
	return h
}
