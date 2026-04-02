package app

import (
	"github.com/linxlib/fw/annotation"
	"github.com/linxlib/fw/astp"
	"github.com/linxlib/fw/middleware"
)

func (e *Engine) matchMiddleware(doc *astp.CommentGroup, scope middleware.Scope) []middleware.Bound {
	anns := annotation.FromDoc(doc)
	index := make(map[string]int)
	var out []middleware.Bound
	for _, parsed := range anns {
		name := parsed.Name
		mw, ok := e.middlewares[name]
		if !ok {
			continue
		}
		spec := mw.Spec()
		if spec.Scope != middleware.ScopeBoth && spec.Scope != scope {
			continue
		}
		bound := middleware.Bound{MW: mw, Args: middleware.AnnotationArgs{Args: parsed.Args, KV: parsed.KV}}
		if i, exists := index[name]; exists {
			out[i] = bound
			continue
		}
		index[name] = len(out)
		out = append(out, bound)
	}
	return out
}

func resolveScopedMiddleware(controllerLevel []middleware.Bound, methodLevel []middleware.Bound) ([]middleware.Bound, []middleware.Bound) {
	override := make(map[string]struct{})
	for _, item := range methodLevel {
		override[item.MW.Spec().Name] = struct{}{}
	}
	var ctrl []middleware.Bound
	for _, item := range controllerLevel {
		if _, ok := override[item.MW.Spec().Name]; ok {
			continue
		}
		ctrl = append(ctrl, item)
	}
	return ctrl, methodLevel
}

func splitStage(items []middleware.Bound) (before []middleware.Bound, after []middleware.Bound) {
	for _, item := range items {
		stage := item.MW.Spec().Stage
		switch stage {
		case middleware.StageAfter:
			after = append(after, item)
		case middleware.StageBoth:
			before = append(before, item)
			after = append(after, item)
		default:
			before = append(before, item)
		}
	}
	return before, after
}
