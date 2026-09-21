package app

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/fasthttp/router"
	"github.com/linxlib/fw/v2/annotation"
	"github.com/linxlib/fw/v2/astp"
	"github.com/linxlib/fw/v2/config"
	ctxpkg "github.com/linxlib/fw/v2/context"
	"github.com/linxlib/fw/v2/inject"
	"github.com/linxlib/fw/v2/logger"
	"github.com/linxlib/fw/v2/middleware"
	"github.com/linxlib/fw/v2/openapi"
	"github.com/linxlib/fw/v2/response"
	"github.com/pterm/pterm"
	"github.com/valyala/fasthttp"
)

type Engine struct {
	cfg             *EngineConfig
	log             *logger.Logger
	loader          *config.Config
	router          *router.Router
	globalContainer inject.Injector
	responseManager *response.Manager
	project         *astp.Project
	autoRegistered  bool

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

// New 创建引擎并加载配置.
//
// configPath 为 YAML 配置文件路径, 传空串时使用 config 包缺省的 config/config.yaml.
// 配置按 app/config.go 里 EngineConfig 的 inject tag 注入, 文件中缺失的段保留缺省值.
func New(configPath string) (*Engine, error) {
	opts := &config.Option{}
	if strings.TrimSpace(configPath) != "" {
		opts.Files = []string{configPath}
	}
	loader := config.New(opts)

	cfg := DefaultEngineConfig()
	if err := loader.LoadByTags(&cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	logg, err := logger.New(cfg.Log.Level, cfg.Log.Output, cfg.Log.FilePath, cfg.Log.EnableFile)
	if err != nil {
		return nil, err
	}
	return &Engine{
		cfg:             &cfg,
		loader:          loader,
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

// EnableConfigReload 开启配置文件热重载(缺省关闭, 需显式开启).
//
// interval 为轮询间隔, <=0 时按 1s 处理. 开启后按顶层 section 增量重载: 只有内容
// 发生变化的 section 对应的内存配置被重写, config 包同时在标准输出打印
// "config: reload detected, changed sections: [...]" 提示本次变化.
// 路由表与监听地址在 Build/ListenAndServe 时已确定, 重载不会重建它们, 因此应在
// New 之后、ListenAndServe 之前调用.
func (e *Engine) EnableConfigReload(interval time.Duration) {
	if e == nil || e.loader == nil {
		return
	}
	if interval <= 0 {
		interval = time.Second
	}
	// 先挂回调再启动轮询, 避免轮询 goroutine 读到未初始化的回调字段.
	e.loader.AutoReloadCallback = e.onConfigReload
	e.loader.StartAutoReload(interval)
	if e.log != nil {
		e.log.Infof("config auto reload enabled, interval %s", interval)
	}
}

// onConfigReload 是配置热重载回调: 某个 section 变化后由 config 包在锁外调用.
// 这里只做观测性日志, 是否采用新值由各读取方自行决定.
func (e *Engine) onConfigReload(key string, target any) {
	if e.log == nil {
		return
	}
	e.log.Infof("config reloaded: section %q updated", key)
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
	e.injectMiddlewareDependencies(mw, spec.Name)
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
	e.autoInjectFields(v, e.globalContainer)
	t := v.Type()
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	e.controllers[t.Name()] = v
	_ = e.globalContainer.Apply(controller)
}

func (e *Engine) autoInjectFields(v reflect.Value, container inject.Injector) {
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

		mapped := container.Get(field.Type())
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

func (e *Engine) injectMiddlewareDependencies(mw middleware.Middleware, name string) {
	container := inject.New()
	container.SetParent(e.globalContainer)
	section := e.middlewareConfig(name)
	container.Map(section)
	e.autoInjectFields(reflect.ValueOf(mw), container)
	_ = container.Apply(mw)
}

// middlewareConfig 返回某个中间件对应的配置段(按中间件名小写匹配).
// 配置文件里没有对应段时返回空 Section, 因此中间件无需判空.
func (e *Engine) middlewareConfig(name string) *config.Section {
	key := strings.ToLower(strings.TrimSpace(name))
	if e.cfg != nil && e.cfg.Middlewares != nil {
		if section, ok := e.cfg.Middlewares[key]; ok {
			cpy := section
			return &cpy
		}
	}
	empty := config.NewSection(nil)
	return &empty
}

// EmbedProject sets pre-generated AST metadata (the contents of .astp.json).
// When set, Build() uses this data instead of parsing Go source files at runtime.
//
// Typical usage with go:generate + go:embed:
//
//	//go:generate go run github.com/linxlib/fw/v2/astp/cmd/astp -exported .
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
	if err := e.applyAutoRegistrars(); err != nil {
		return err
	}
	if err := e.collectRoutes(); err != nil {
		return err
	}
	if err := e.registerRoutes(); err != nil {
		return err
	}
	e.printGlobalMiddlewares()
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

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port %d is already in use: %w", e.cfg.Server.Port, err)
	}
	ln.Close()

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
				// 泛型实参绑定: 让 schema 能展开真实的实体类型而不是 {}.
				typeArgs := astp.RecvTypeArgs(q, typ, m)
				respSchema := responseSchemaForMethod(q, m, typeArgs)
				methodDesc := entityDescription(m.Name, m.Doc)
				methodMW := e.matchMiddleware(m.Doc, middleware.ScopeMethod)
				ctrlIgnore := parseIgnoreList(typ.Doc)
				methodIgnore := parseIgnoreList(m.Doc)
				ignoreSet := mergeIgnoreSets(ctrlIgnore, methodIgnore)
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
					hints := buildParamHints(q, m, pathParamSet, typeArgs)
					opArgs, opBody := buildOpenAPIForMethod(q, m, hints, typeArgs)
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
		e.router.Handle(rt.Method, routerPath(rt.Path), func(raw *fasthttp.RequestCtx) {
			start := time.Now()
			if e.cfg.Log.RequestEnabled {
				defer func() {
					status := raw.Response.StatusCode()
					if status == 0 {
						status = fasthttp.StatusOK
					}
					e.log.Infof("%s %s -> %d (%s)", string(raw.Method()), string(raw.Path()), status, time.Since(start))
				}()
			}
			if e.cfg.Recovery.Enabled {
				defer func() {
					recovered := recover()
					if recovered == nil {
						return
					}

					panicText, stackPayload, stackTree := buildRecoveryReport(recovered)
					e.log.Errorf("panic recovered: %s", panicText)
					e.log.Errorf("stack:")
					for _, line := range stackTree {
						e.log.Errorf("%s", line)
					}

					var data any
					if e.cfg.Recovery.ReturnStackToBody {
						data = map[string]any{
							"panic": panicText,
							"stack": stackPayload,
						}
					}
					_ = ctxpkg.New(raw, e.responseManager).StatusServerError().Data(data)
				}()
			}

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
				_ = c.StatusParamError().Send()
				return
			}
			c.SetContainer(reqContainer)
			rawResponse := hasRawResponseAnnotation(rtCopy.AstMethod)
			h := middleware.Chain(func(ctx ctxpkg.Context) error {
				results, err := invokeMethod(reqContainer, rtCopy.AstMethod, rtCopy.HandlerValue, rtCopy.ParamHints)
				if err != nil {
					return err
				}
				data, err := extractMethodResponse(results)
				if err != nil {
					return err
				}
				if responseWritten(raw) {
					return nil
				}
				if data != nil {
					if rawResponse {
						return e.responseManager.WriteRaw(raw, fasthttp.StatusOK, data)
					}
					return ctx.StatusOK().Data(data)
				}
				if !responseWritten(raw) {
					return ctx.StatusOK().Send()
				}
				return nil
			}, rtCopy.GlobalMW, rtCopy.CtrlMW, rtCopy.MethodMW)
			if err := h(c); err != nil {
				e.log.Errorf("handler failed: %v", err)
				if !responseWritten(raw) {
					_ = c.StatusServerError().Send()
				}
			}
		})
	}
	return nil
}

func (e *Engine) printRoutes() {
	tree := make(map[string][]routeDef)
	for _, rt := range e.routes {
		tree[rt.Method] = append(tree[rt.Method], rt)
	}
	methods := make([]string, 0, len(tree))
	for method := range tree {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	for _, method := range methods {
		e.log.Infof("%s", colorizeHTTPMethod(method))
		routes := tree[method]
		sort.Slice(routes, func(i, j int) bool {
			if routes[i].Path == routes[j].Path {
				return routes[i].Controller+"."+routes[i].Handler < routes[j].Controller+"."+routes[j].Handler
			}
			return routes[i].Path < routes[j].Path
		})
		for _, rt := range routes {
			mwNames := routeMiddlewareNames(rt)
			e.log.Infof("  |- %s [mw: %s]", rt.Path, strings.Join(mwNames, ", "))
		}
	}
}

func (e *Engine) printGlobalMiddlewares() {
	names := make([]string, 0, len(e.globalUse))
	seen := make(map[string]struct{})
	for _, bound := range e.globalUse {
		name := strings.TrimSpace(bound.MW.Spec().Name)
		if name != "" {
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		e.log.Infof("global middlewares: (none)")
		return
	}
	e.log.Infof("global middlewares: %s", strings.Join(names, ", "))
}

func routeMiddlewareNames(rt routeDef) []string {
	names := make([]string, 0, len(rt.GlobalMW)+len(rt.CtrlMW)+len(rt.MethodMW))
	seen := make(map[string]struct{})
	for _, layer := range [][]middleware.Bound{rt.GlobalMW, rt.CtrlMW, rt.MethodMW} {
		for _, bound := range layer {
			name := strings.TrimSpace(bound.MW.Spec().Name)
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return []string{"(none)"}
	}
	return names
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

type recoveryFrame struct {
	Func string `json:"func"`
	File string `json:"file"`
	Line int    `json:"line"`
}

func buildRecoveryReport(recovered any) (string, []recoveryFrame, []string) {
	panicText := fmt.Sprint(recovered)
	frames := collectRecoveryFrames()
	return panicText, frames, formatRecoveryTree(frames)
}

func collectRecoveryFrames() []recoveryFrame {
	pcs := make([]uintptr, 64)
	n := runtime.Callers(3, pcs)
	if n == 0 {
		return nil
	}

	iter := runtime.CallersFrames(pcs[:n])
	frames := make([]recoveryFrame, 0, 6)
	fallback := make([]recoveryFrame, 0, 6)
	for {
		frame, more := iter.Next()
		if shouldSkipRecoveryFrame(frame) {
			if !more {
				break
			}
			continue
		}

		item := recoveryFrame{Func: frame.Function, File: filepath.ToSlash(frame.File), Line: frame.Line}
		if isProjectRecoveryFrame(frame) {
			frames = append(frames, item)
		} else if len(fallback) < 6 {
			fallback = append(fallback, item)
		}

		if len(frames) >= 6 {
			break
		}
		if !more {
			break
		}
	}

	if len(frames) == 0 {
		return fallback
	}
	return frames
}

func shouldSkipRecoveryFrame(frame runtime.Frame) bool {
	fn := frame.Function
	if fn == "" {
		return true
	}
	if strings.HasPrefix(fn, "runtime.") {
		return true
	}
	if strings.Contains(filepath.ToSlash(frame.File), "/runtime/") && strings.Contains(fn, "panic") {
		return true
	}
	if strings.Contains(fn, "buildRecoveryReport") || strings.Contains(fn, "collectRecoveryFrames") || strings.Contains(fn, "formatRecoveryTree") {
		return true
	}
	if strings.Contains(fn, "registerRoutes.func1.2") {
		return true
	}
	return false
}

func isProjectRecoveryFrame(frame runtime.Frame) bool {
	file := filepath.ToSlash(frame.File)
	return strings.Contains(frame.Function, "github.com/linxlib/fw/v2/") || strings.Contains(file, "/fw/")
}

func formatRecoveryTree(frames []recoveryFrame) []string {
	if len(frames) == 0 {
		return []string{"(no stack frames available)"}
	}
	out := make([]string, 0, len(frames)*2)
	for i, frame := range frames {
		branch := "├─"
		indent := "│  "
		if i == len(frames)-1 {
			branch = "└─"
			indent = "   "
		}
		out = append(out, fmt.Sprintf("%s %d. %s", branch, i+1, frame.Func))
		out = append(out, fmt.Sprintf("%s%s:%d", indent, frame.File, frame.Line))
	}
	return out
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

// joinPath 拼接控制器基础路径与方法注解路径.
//
// 指向控制器根路径的注解(如 "@GET /")收敛为不带尾斜杠的集合路径: /users + / -> /users.
// 这样集合路由与 /users/{id} 形态一致, 也不会让客户端被尾斜杠重定向弹来弹去
// (此前是 /users/, GET /users 会被 301 到 /users/).
func joinPath(base, sub string) string {
	base = normalizePath(base)
	sub = normalizePath(sub)
	if base == "/" {
		return sub
	}
	if sub == "/" {
		return strings.TrimSuffix(base, "/")
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

// routerPath 把 fw 的 ":name" 路由语法转换成 fasthttp/router 的 "{name}" 语法.
//
// fw 的注解与 pathParamNames 一直使用 ":name"(如 @GET /users/:id), 而
// github.com/fasthttp/router 从 v1.5 起改用 "{name}", ":name" 会被当成普通静态段,
// 导致所有参数路由静默失配(返回 404). 这里在注册边界上做一次转换, 保持对外语法不变.
func routerPath(path string) string {
	if !strings.Contains(path, ":") {
		return path
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			parts[i] = "{" + part[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
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
