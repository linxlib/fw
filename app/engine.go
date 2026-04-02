package app

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/fasthttp/router"
	"github.com/linxlib/fw/annotation"
	"github.com/linxlib/fw/astp"
	"github.com/linxlib/fw/config"
	ctxpkg "github.com/linxlib/fw/context"
	"github.com/linxlib/fw/inject"
	"github.com/linxlib/fw/logger"
	"github.com/linxlib/fw/middleware"
	"github.com/linxlib/fw/openapi"
	"github.com/linxlib/fw/response"
	"github.com/pterm/pterm"
	"github.com/valyala/fasthttp"
)

type Engine struct {
	cfg             config.Config
	log             *logger.Logger
	router          *router.Router
	globalContainer inject.Injector
	responseManager *response.Manager
	project         *astp.Project

	controllers  map[string]reflect.Value
	middlewares  map[string]middleware.Middleware
	globalUse    []middleware.Bound
	routes       []routeDef
	embeddedASTP []byte // pre-generated AST metadata (JSON), set via EmbedProject()
}

type routeDef struct {
	Method            string
	Path              string
	Controller        string
	ControllerDesc    string
	Handler           string
	HandlerDesc       string
	HandlerValue      reflect.Value
	ParamNames        []string
	ParamHints        map[string]ParamHint
	OpenAPIArgs       []openapi.Parameter
	OpenAPIBody       *openapi.RequestBody
	OpenAPIRespSchema *openapi.Schema
	OpenAPISecurity   []openapi.SecurityRequirement
	AstMethod         *astp.Func
	GlobalMW          []middleware.Bound
	CtrlMW            []middleware.Bound
	MethodMW          []middleware.Bound
}

func New(configPath string) (*Engine, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	logg, err := logger.New(cfg.Log.Level, cfg.Log.Output, cfg.Log.FilePath, cfg.Log.EnableFile)
	if err != nil {
		return nil, err
	}
	return &Engine{
		cfg:             cfg,
		log:             logg,
		router:          router.New(),
		globalContainer: inject.New(),
		responseManager: response.NewManager(nil, nil),
		controllers:     make(map[string]reflect.Value),
		middlewares:     make(map[string]middleware.Middleware),
	}, nil
}

func (e *Engine) SetResponse(formatter response.Formatter, encoder response.Encoder) {
	e.responseManager = response.NewManager(formatter, encoder)
}

func (e *Engine) Container() inject.Injector {
	return e.globalContainer
}

func (e *Engine) RegisterMiddleware(mw middleware.Middleware) {
	if mw == nil {
		return
	}
	spec := mw.Spec()
	if spec.Name == "" {
		return
	}
	e.middlewares[spec.Name] = mw
}

func (e *Engine) Use(mw middleware.Middleware) {
	e.RegisterMiddleware(mw)
	e.globalUse = append(e.globalUse, middleware.Bound{MW: mw})
}

func (e *Engine) RegisterController(controller any) {
	v := reflect.ValueOf(controller)
	if !v.IsValid() {
		return
	}
	e.autoInjectControllerFields(v)
	t := v.Type()
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	e.controllers[t.Name()] = v
	_ = e.globalContainer.Apply(controller)
}

func (e *Engine) autoInjectControllerFields(v reflect.Value) {
	if !v.IsValid() {
		return
	}
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanSet() || !field.IsZero() {
			continue
		}

		mapped := e.globalContainer.Get(field.Type())
		if !mapped.IsValid() {
			continue
		}

		if mapped.Type().AssignableTo(field.Type()) {
			field.Set(mapped)
			continue
		}
		if mapped.Type().ConvertibleTo(field.Type()) {
			field.Set(mapped.Convert(field.Type()))
			continue
		}
	}
}

// EmbedProject sets pre-generated AST metadata (the contents of .astp.json).
// When set, Build() uses this data instead of parsing Go source files at runtime.
//
// Typical usage with go:generate + go:embed:
//
//	//go:generate go run github.com/linxlib/fw/astp/cmd/astp -exported .
//	//go:embed .astp.json
//	var astpData []byte
//
//	func main() {
//	    e, _ := app.New("")
//	    e.EmbedProject(astpData)
//	    // ...
//	}
func (e *Engine) EmbedProject(data []byte) {
	e.embeddedASTP = data
}

func (e *Engine) Build() error {
	if err := e.loadProject(); err != nil {
		return err
	}
	if err := e.collectRoutes(); err != nil {
		return err
	}
	if err := e.registerRoutes(); err != nil {
		return err
	}
	e.printRoutes()
	if e.cfg.OpenAPI.Enabled {
		opi := make([]openapi.RouteInfo, 0, len(e.routes))
		secSchemes := make(map[string]openapi.SecurityScheme)
		for _, rt := range e.routes {
			opi = append(opi, openapi.RouteInfo{
				Method:               rt.Method,
				Path:                 rt.Path,
				OperationID:          operationIDForRoute(rt),
				OperationDescription: rt.HandlerDesc,
				TagName:              rt.Controller,
				TagDescription:       rt.ControllerDesc,
				Parameters:           rt.OpenAPIArgs,
				RequestBody:          rt.OpenAPIBody,
				ResponseSchema:       rt.OpenAPIRespSchema,
				Security:             rt.OpenAPISecurity,
			})
			collectSecuritySchemes(secSchemes, rt.GlobalMW, rt.CtrlMW, rt.MethodMW)
		}
		if err := openapi.Generate(e.cfg.OpenAPI.Output, e.cfg.OpenAPI.Title, e.cfg.OpenAPI.Version, opi, secSchemes); err != nil {
			return err
		}
		openapi.RegisterDocsRoutes(e.router, e.cfg.OpenAPI.Output)
		e.log.Infof("openapi generated: %s", e.cfg.OpenAPI.Output)
	}
	return nil
}

func (e *Engine) ListenAndServe() error {
	if err := e.Build(); err != nil {
		return err
	}
	addr := net.JoinHostPort(e.cfg.Server.Host, fmt.Sprintf("%d", e.cfg.Server.Port))
	e.log.Infof("server listening on %s", addr)
	if e.cfg.OpenAPI.Enabled {
		e.log.Infof("swagger ui: %s", e.swaggerDocsURL())
	}
	return fasthttp.ListenAndServe(addr, e.router.Handler)
}

func (e *Engine) swaggerDocsURL() string {
	host := strings.TrimSpace(e.cfg.Server.Host)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	hostPort := net.JoinHostPort(host, fmt.Sprintf("%d", e.cfg.Server.Port))
	return "http://" + hostPort + "/docs"
}

func (e *Engine) loadProject() error {
	// 1. Resolve project directory and prefer live parsing when source code exists.
	projectDir := e.cfg.ProjectDir
	if projectDir == "" {
		projectDir = "."
	}
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}
	if ok, err := hasGoSources(abs); err == nil && ok {
		p := astp.NewParser()
		proj, err := p.ParseProject(abs)
		if err != nil {
			return err
		}
		e.project = proj
		e.log.Infof("loaded project metadata via live parsing from %s", abs)
		return nil
	}

	// 2. Try to load pre-generated .astp.json from project dir.
	astpFile := filepath.Join(abs, astp.DefaultOutputFile)
	if _, err := os.Stat(astpFile); err == nil {
		proj, err := astp.Load(astpFile)
		if err != nil {
			return fmt.Errorf("load %s: %w", astpFile, err)
		}
		e.project = proj
		e.log.Infof("loaded project metadata from %s", astpFile)
		return nil
	}

	// 3. Fall back to embedded AST data (deployment mode — no source required).
	if len(e.embeddedASTP) > 0 {
		proj, err := astp.LoadFromBytes(e.embeddedASTP)
		if err != nil {
			return fmt.Errorf("load embedded astp data: %w", err)
		}
		e.project = proj
		e.log.Infof("loaded project metadata from embedded data")
		return nil
	}

	return fmt.Errorf("no Go source files in %s, and neither %s nor embedded metadata is available", abs, astp.DefaultOutputFile)
}

func hasGoSources(root string) (bool, error) {
	if _, err := os.Stat(root); err != nil {
		return false, err
	}
	found := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".go") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.SkipAll) {
		return false, err
	}
	return found, nil
}

func (e *Engine) collectRoutes() error {
	if e.project == nil {
		return errors.New("project metadata not loaded")
	}
	q := astp.NewQuery(e.project)
	for _, pkg := range e.project.Packages {
		for _, typ := range pkg.Types {
			ctrlValue, ok := e.controllers[typ.Name]
			if !ok {
				continue
			}
			ctrlDesc := entityDescription(typ.Name, typ.Doc)
			ctrlAnn := annotation.FromDoc(typ.Doc)
			basePath := "/"
			for _, a := range ctrlAnn {
				if strings.EqualFold(a.Name, "Route") && len(a.Args) > 0 {
					basePath = normalizePath(a.Args[0])
				}
			}
			ctrlMW := e.matchMiddleware(typ.Doc, middleware.ScopeController)
			for _, m := range typ.Methods {
				methodVal := ctrlValue.MethodByName(m.Name)
				if !methodVal.IsValid() {
					continue
				}
				routes := parseHTTPRoutes(m.Doc)
				if len(routes) == 0 {
					continue
				}
				methodDesc := entityDescription(m.Name, m.Doc)
				respSchema := responseSchemaForMethod(q, m)
				methodMW := e.matchMiddleware(m.Doc, middleware.ScopeMethod)
				ignoreSet := parseIgnoreList(m.Doc)
				ctrlResolved, methodResolved := resolveScopedMiddleware(ctrlMW, methodMW, ignoreSet)
				globalResolved := filterIgnored(e.globalUse, ignoreSet)
				seen := make(map[string]struct{})
				for _, rt := range routes {
					fullPath := joinPath(basePath, rt.Path)
					paramNames := pathParamNames(fullPath)
					pathParamSet := make(map[string]struct{}, len(paramNames))
					for _, name := range paramNames {
						pathParamSet[name] = struct{}{}
					}
					hints := buildParamHints(q, m, pathParamSet)
					opArgs, opBody := buildOpenAPIForMethod(q, m, hints)
					key := rt.Method + " " + fullPath
					if _, exists := seen[key]; exists {
						return fmt.Errorf("duplicate route annotation in method %s.%s: %s", typ.Name, m.Name, key)
					}
					seen[key] = struct{}{}
					e.routes = append(e.routes, routeDef{
						Method:            rt.Method,
						Path:              fullPath,
						Controller:        typ.Name,
						ControllerDesc:    ctrlDesc,
						Handler:           m.Name,
						HandlerDesc:       methodDesc,
						HandlerValue:      methodVal,
						ParamNames:        paramNames,
						ParamHints:        hints,
						OpenAPIArgs:       opArgs,
						OpenAPIBody:       opBody,
						OpenAPIRespSchema: respSchema,
						OpenAPISecurity:   collectSecurityRequirements(globalResolved, ctrlResolved, methodResolved),
						AstMethod:         m,
						GlobalMW:          globalResolved,
						CtrlMW:            ctrlResolved,
						MethodMW:          methodResolved,
					})
				}
			}
		}
	}
	return nil
}

func (e *Engine) registerRoutes() error {
	seen := make(map[string]struct{})
	for _, rt := range e.routes {
		key := rt.Method + " " + rt.Path
		if _, exists := seen[key]; exists {
			return fmt.Errorf("route already registered: %s", key)
		}
		seen[key] = struct{}{}
		rtCopy := rt
		e.router.Handle(rt.Method, rt.Path, func(raw *fasthttp.RequestCtx) {
			c := ctxpkg.New(raw, e.responseManager)
			params := make(map[string]string, len(rtCopy.ParamNames))
			for _, name := range rtCopy.ParamNames {
				v := raw.UserValue(name)
				if v == nil {
					continue
				}
				params[name] = fmt.Sprintf("%v", v)
			}
			c.SetRouteParams(params)
			reqContainer, err := buildRequestContainer(c, e.globalContainer, rtCopy.AstMethod, rtCopy.ParamHints)
			if err != nil {
				e.log.Errorf("build request container failed: %v", err)
				_ = c.Respond(fasthttp.StatusBadRequest, 40001, "invalid request", nil)
				return
			}
			c.SetContainer(reqContainer)
			h := middleware.Chain(func(ctx ctxpkg.Context) error {
				_, err := invokeMethod(reqContainer, rtCopy.AstMethod, rtCopy.HandlerValue, rtCopy.ParamHints)
				if err != nil {
					return err
				}
				if raw.Response.StatusCode() == 0 {
					return ctx.Respond(fasthttp.StatusOK, 0, "ok", nil)
				}
				return nil
			}, rtCopy.GlobalMW, rtCopy.CtrlMW, rtCopy.MethodMW)
			if err := h(c); err != nil {
				e.log.Errorf("handler failed: %v", err)
				if raw.Response.StatusCode() == 0 {
					_ = c.Respond(fasthttp.StatusInternalServerError, 50000, "internal error", nil)
				}
			}
		})
	}
	return nil
}

func (e *Engine) printRoutes() {
	tree := make(map[string][]string)
	for _, rt := range e.routes {
		tree[rt.Method] = append(tree[rt.Method], rt.Path)
	}
	methods := make([]string, 0, len(tree))
	for method := range tree {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	for _, method := range methods {
		e.log.Infof("%s", colorizeHTTPMethod(method))
		paths := tree[method]
		sort.Strings(paths)
		for _, path := range paths {
			e.log.Infof("  |- %s", path)
		}
	}
}

func colorizeHTTPMethod(method string) string {
	switch method {
	case fasthttp.MethodGet:
		return pterm.FgGreen.Sprintf("%s", method)
	case fasthttp.MethodPost:
		return pterm.FgLightCyan.Sprintf("%s", method)
	case fasthttp.MethodPut:
		return pterm.FgYellow.Sprintf("%s", method)
	case fasthttp.MethodDelete:
		return pterm.FgRed.Sprintf("%s", method)
	case fasthttp.MethodPatch:
		return pterm.FgLightYellow.Sprintf("%s", method)
	default:
		return method
	}
}

func parseHTTPRoutes(doc *astp.CommentGroup) []httpRoute {
	anns := annotation.FromDoc(doc)
	var routes []httpRoute
	for _, ann := range anns {
		method := strings.ToUpper(strings.TrimSpace(ann.Name))
		if !isHTTPMethod(method) {
			continue
		}
		if len(ann.Args) == 0 {
			continue
		}
		routes = append(routes, httpRoute{Method: method, Path: ann.Args[0]})
	}
	return routes
}

type httpRoute struct {
	Method string
	Path   string
}

func isHTTPMethod(s string) bool {
	switch s {
	case fasthttp.MethodGet, fasthttp.MethodPost, fasthttp.MethodPut, fasthttp.MethodDelete, fasthttp.MethodPatch, fasthttp.MethodOptions, fasthttp.MethodHead:
		return true
	default:
		return false
	}
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func joinPath(base, sub string) string {
	base = normalizePath(base)
	sub = normalizePath(sub)
	if base == "/" {
		return sub
	}
	return strings.TrimSuffix(base, "/") + sub
}

func pathParamNames(path string) []string {
	parts := strings.Split(path, "/")
	var out []string
	for _, part := range parts {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			out = append(out, part[1:])
		}
	}
	return out
}

func operationIDForRoute(rt routeDef) string {
	path := strings.Trim(rt.Path, "/")
	if path == "" {
		path = "root"
	}
	return sanitizeOperationIDPart(rt.Controller) + "_" +
		sanitizeOperationIDPart(rt.Handler) + "_" +
		strings.ToLower(rt.Method) + "_" +
		sanitizeOperationIDPart(path)
}

func sanitizeOperationIDPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "x"
	}
	var b strings.Builder
	b.Grow(len(s))
	lastUnderscore := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "x"
	}
	return out
}
