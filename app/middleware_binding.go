package app

import (
	"strings"

	"github.com/linxlib/fw/v2/annotation"
	"github.com/linxlib/fw/v2/astp"
	"github.com/linxlib/fw/v2/middleware"
	"github.com/linxlib/fw/v2/openapi"
)

func (e *Engine) matchMiddleware(doc *astp.CommentGroup, scope middleware.Scope) []middleware.Bound {
	annotations := annotation.FromDoc(doc)
	index := make(map[string]int)
	var out []middleware.Bound
	for _, parsed := range annotations {
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

// parseIgnoreList extracts middleware names from @Ignore annotations on a method.
// e.g. @Ignore(Authorization, Log) → {"Authorization": {}, "Log": {}}
// Multiple @Ignore annotations are merged.
func parseIgnoreList(doc *astp.CommentGroup) map[string]struct{} {
	annotations := annotation.FromDoc(doc)
	out := make(map[string]struct{})
	for _, parsed := range annotations {
		if parsed.Name != "Ignore" {
			continue
		}
		for _, arg := range parsed.Args {
			if arg != "" {
				out[arg] = struct{}{}
			}
		}
	}
	return out
}

// filterIgnored removes any Bound entries whose middleware name appears in the ignore set.
func filterIgnored(list []middleware.Bound, ignore map[string]struct{}) []middleware.Bound {
	if len(ignore) == 0 {
		return list
	}
	if hasIgnored(ignore, "Global") {
		return nil
	}
	var out []middleware.Bound
	for _, item := range list {
		if hasIgnored(ignore, item.MW.Spec().Name) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func mergeIgnoreSets(items ...map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{})
	for _, item := range items {
		for name := range item {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			out[name] = struct{}{}
		}
	}
	return out
}

func hasIgnored(ignore map[string]struct{}, name string) bool {
	if len(ignore) == 0 {
		return false
	}
	if _, ok := ignore[name]; ok {
		return true
	}
	for key := range ignore {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

func resolveScopedMiddleware(controllerLevel []middleware.Bound, methodLevel []middleware.Bound, ignore map[string]struct{}) ([]middleware.Bound, []middleware.Bound) {
	override := make(map[string]struct{})
	for _, item := range methodLevel {
		override[item.MW.Spec().Name] = struct{}{}
	}
	var ctrl []middleware.Bound
	for _, item := range controllerLevel {
		name := item.MW.Spec().Name
		if _, ok := override[name]; ok {
			continue
		}
		if _, skip := ignore[name]; skip {
			continue
		}
		ctrl = append(ctrl, item)
	}
	return ctrl, methodLevel
}

// collectSecurityRequirements inspects all bound middleware layers (global, controller, method)
// and returns OpenAPI security requirements for any middleware implementing SecurityProvider.
func collectSecurityRequirements(globalMW, ctrlMW, methodMW []middleware.Bound) []openapi.SecurityRequirement {
	seen := make(map[string]struct{})
	var reqs []openapi.SecurityRequirement
	for _, list := range [][]middleware.Bound{globalMW, ctrlMW, methodMW} {
		for _, bound := range list {
			sp, ok := bound.MW.(middleware.SecurityProvider)
			if !ok {
				continue
			}
			scheme := sp.SecurityScheme()
			if scheme.Name == "" {
				continue
			}
			if _, exists := seen[scheme.Name]; exists {
				continue
			}
			seen[scheme.Name] = struct{}{}
			reqs = append(reqs, openapi.SecurityRequirement{scheme.Name: {}})
		}
	}
	return reqs
}

// collectSecuritySchemes extracts OpenAPI SecurityScheme definitions from bound middlewares
// and merges them into the provided map (keyed by scheme name).
func collectSecuritySchemes(out map[string]openapi.SecurityScheme, layers ...[]middleware.Bound) {
	for _, layer := range layers {
		for _, bound := range layer {
			sp, ok := bound.MW.(middleware.SecurityProvider)
			if !ok {
				continue
			}
			mwScheme := sp.SecurityScheme()
			if mwScheme.Name == "" {
				continue
			}
			if _, exists := out[mwScheme.Name]; exists {
				continue
			}
			s := openapi.SecurityScheme{
				Type:        string(mwScheme.Type),
				Description: mwScheme.Description,
			}
			switch mwScheme.Type {
			case middleware.SecurityHTTP:
				s.Scheme = mwScheme.Scheme
				s.BearerFormat = mwScheme.BearerFormat
			case middleware.SecurityAPIKey:
				s.In = string(mwScheme.In)
				s.Name = mwScheme.FieldName
			}
			out[mwScheme.Name] = s
		}
	}
}
