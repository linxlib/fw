package middleware

import (
	"github.com/linxlib/fw/context"
)

type Handler func(context.Context) error

type Middleware interface {
	Spec() AnnotationSpec
	Before(ctx context.Context, args AnnotationArgs) error
	After(ctx context.Context, args AnnotationArgs) error
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

type Bound struct {
	MW   Middleware
	Args AnnotationArgs
}

func Chain(handler Handler, globalBefore []Bound, controllerBefore []Bound, methodBefore []Bound, methodAfter []Bound, controllerAfter []Bound, globalAfter []Bound) Handler {
	return func(ctx context.Context) error {
		if err := runBefore(ctx, globalBefore); err != nil {
			return err
		}
		if err := runBefore(ctx, controllerBefore); err != nil {
			return err
		}
		if err := runBefore(ctx, methodBefore); err != nil {
			return err
		}
		if err := handler(ctx); err != nil {
			return err
		}
		if err := runAfter(ctx, methodAfter); err != nil {
			return err
		}
		if err := runAfter(ctx, controllerAfter); err != nil {
			return err
		}
		if err := runAfter(ctx, globalAfter); err != nil {
			return err
		}
		return nil
	}
}

func runBefore(ctx context.Context, list []Bound) error {
	for _, item := range list {
		if err := item.MW.Before(ctx, item.Args); err != nil {
			return err
		}
	}
	return nil
}

func runAfter(ctx context.Context, list []Bound) error {
	for _, item := range list {
		if err := item.MW.After(ctx, item.Args); err != nil {
			return err
		}
	}
	return nil
}
