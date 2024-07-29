package fw

import (
	"fmt"
	"github.com/fasthttp/router"
	"github.com/linxlib/astp"
	"github.com/linxlib/fw/attribute"
	"github.com/linxlib/fw/binding"
	"github.com/linxlib/fw/internal"
	"github.com/linxlib/fw/internal/json"
	"github.com/linxlib/fw/options"
	"github.com/linxlib/fw/types"
	"github.com/linxlib/inject"
	"github.com/olekukonko/tablewriter"
	"github.com/valyala/fasthttp"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

const Version = "1.0.0-beta"

func New() *Server {
	s := &Server{
		Injector:   inject.New(),
		router:     router.New(),
		server:     &fasthttp.Server{},
		option:     new(options.ServerOption),
		parser:     astp.NewParser(),
		middleware: NewMiddlewareContainer(),
		logger:     &Logger{},
	}
	options.ReadConfig(s.option)
	if s.option.Debug {
		bs, _ := json.MarshalIndent(s.option, "", "    ")
		fmt.Println(string(bs))
	}

	s.Map(s.option)
	s.Map(s)
	if !internal.FileIsExist(s.option.AstFile) {
		if s.option.Dev {
			parser := astp.NewParser()
			parser.Parse()
			_ = parser.WriteOut(s.option.AstFile)
		} else {
			panic(fmt.Sprintf("%s not found, please generate it first!", s.option.AstFile))
		}
	}

	s.parser.Load(s.option.AstFile)
	s.tw = tablewriter.NewWriter(os.Stdout)
	s.tw.SetHeader([]string{"Controller", "Method", "Route", "Method", "Signature"})

	s.tw.SetRowLine(true)
	s.tw.SetCenterSeparator("|")
	return s
}

type Server struct {
	inject.Injector
	server     *fasthttp.Server
	router     *router.Router
	option     *options.ServerOption
	parser     *astp.Parser
	tw         *tablewriter.Table
	middleware *MiddlewareContainer
	logger     types.ILogger
	once       sync.Once
	midGlobals []IMiddlewareMethod
}

type HandlerFunc = func(*Context)

// wrap the HandlerFunc to fasthttp.RequestHandler
// just create *Context
func (s *Server) wrap(h HandlerFunc) fasthttp.RequestHandler {

	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()
		c := newContext(ctx, s)
		h(c)
		if s.option.ShowRequestTimeHeader {
			c.ctx.Response.Header.Set(s.option.RequestTimeHeader, time.Since(start).String())
		}
		//fmt.Printf("call spend: %s\n", time.Since(start).String())
	}
}

func (s *Server) addRouteTable(a, b, c, d, e string) {
	if s.option.NoColor {
		s.tw.Append([]string{a, b, c, d, e})
	} else {
		s.tw.Rich([]string{a, b, c, d, e}, []tablewriter.Colors{
			tablewriter.Color(tablewriter.FgBlueColor),
			tablewriter.Color(tablewriter.FgGreenColor),
			tablewriter.Color(tablewriter.FgHiWhiteColor),
			tablewriter.Color(tablewriter.FgHiGreenColor),
			tablewriter.Color(tablewriter.FgHiYellowColor)})
	}
}

func (s *Server) RegisterRoutes(controller ...any) {
	for _, a := range controller {
		s.RegisterRoute(a)
	}
}

const controllerAttr = "Controller"
const controllerRoute = "Route"

func (s *Server) RegisterRoute(controller any) {
	//假定astp已经解析好了整个项目，并通过某种方式还原了Parser内部的值
	// 这里需要通过传入的controller指针 并通过反射方式获取到 controller method param result各种的反射值，并填充到已有的Parser里
	// 这里的指针 通过代码生成的方式
	// 1. 从Parser中获取controller的包路径
	// 2. 生成代码将controller new出来 传到这里来
	//refTyp := reflect.TypeOf(controller)
	refVal := reflect.ValueOf(controller)
	typ := reflect.Indirect(refVal).Type()

	// 处理全局

	s.once.Do(func() {
		s.midGlobals = make([]IMiddlewareMethod, 0)
		m0 := s.handleGlobal(s.option.BasePath)
		for _, item := range m0 {
			s.midGlobals = append(s.midGlobals, item.Middleware)
			if item.Path != "" {
				s.registerRoute(item.Method, item.Path, item.H)
				if !item.IsHide {
					s.addRouteTable("Server", item.Method, item.Path, "Global", "@"+item.Middleware.Name())
				}

			}
		}
	})

	// 遍历代码中所有的 @Controller 标记的结构，按照控制器对待
	s.parser.VisitAllStructs(typ.Name(), func(ctl *astp.Struct) bool {

		if !attribute.HasAttribute(ctl, controllerAttr) {
			return false
		}
		// 第一层路由 【配置文件】
		base := s.option.BasePath
		if base == "" {
			base = "/"
		}
		// 第二层路由 @Route 标记

		if r := attribute.GetStructAttrByName(ctl, controllerRoute); r != nil {
			base = joinRoute(base, r.Value)
		}

		//处理控制器
		m := s.handleCtl(base, ctl)
		mids := make([]IMiddlewareMethod, 0)

		for _, item := range m {
			mids = append(mids, item.Middleware)
			if item.Path != "" {
				s.registerRoute(item.Method, item.Path, item.H)
				if !item.IsHide {
					s.addRouteTable(ctl.Name, item.Method, item.Path, ctl.Name, "@"+item.Middleware.Name())
				}
			}

		}

		//处理控制器方法
		for _, method := range ctl.Methods {
			vm := refVal.MethodByName(method.Name)
			vmt := reflect.TypeOf(vm.Interface())
			for _, param := range method.Params {
				param.SetRType(vmt.In(param.Index))
			}
			method.SetMethod(vm.Interface())

			var hms = make([]string, 0)
			var rps = make([]string, 0)
			for _, command := range attribute.GetMethodAttributes(method) {
				if command.Type == attribute.TypeHttpMethod {
					hms = append(hms, command.Name)
					rps = append(rps, command.Value)
				}
			}
			attrs, call1 := s.handle(method, mids)
			sig := []string{}
			for _, mid := range mids {
				sig = append(sig, "@"+mid.Attribute())
			}
			for _, attr := range attrs {
				sig = append(sig, "@"+attr)
			}

			//TODO: base 和 rp拼接时需要注意下 “/”
			for i, hm := range hms {
				err := s.registerRoute(strings.ToUpper(hm), joinRoute(base, rps[i]), call1)
				if err != nil {
					s.handleError(nil, err)
					continue
				}
				s.addRouteTable(method.Receiver.TypeString, strings.ToUpper(hm), joinRoute(base, rps[i]), method.Name, strings.Join(sig, ","))
			}

		}
		return false
	})
}

func (s *Server) registerRoute(method string, path string, f HandlerFunc) error {
	call1 := s.wrap(f)

	switch method {
	case "POST":
		s.router.POST(path, call1)
	case "GET":
		s.router.GET(path, call1)
	case "DELETE":
		s.router.DELETE(path, call1)
	case "PATCH":
		s.router.PATCH(path, call1)
	case "PUT":
		s.router.PUT(path, call1)
	case "OPTIONS":
		s.router.OPTIONS(path, call1)
	case "HEAD":
		s.router.HEAD(path, call1)
	case "ANY", "WS":
		s.router.ANY(path, call1)
	default:
		return fmt.Errorf("http method:[%v -> %s] not supported", method, path)
	}

	return nil
}

func (s *Server) bind(c *Context, handler *astp.Method) {
	// 准备调用方法需要的参数值
	for _, param := range handler.Params {
		if param.GetRType().AssignableTo(reflect.TypeOf(c)) {
			c.Map(c)
			continue
		}
		//TODO: 是否要兼容 非指针方式声明的参数
		body := reflect.New(param.GetRType().Elem())
		//TODO: 根据请求方法和contentType进行binding
		cmd := attribute.GetLastAttr(param)
		if cmd.Type == attribute.TypeInner {

		} else {
			if strings.ToLower(cmd.Name) == "service" || cmd.Name == "" {

			} else {
				// 对方法参数进行数据映射和校验
				if err := binding.GetByAttr(cmd).Bind(c.GetFastContext(), body.Interface()); err != nil {
					s.handleError(c, err)
				}
			}
			c.Map(body.Interface())
		}

	}
}

func (s *Server) wrapM(handler *astp.Method) HandlerFunc {
	return func(context *Context) {
		s.bind(context, handler)
		_, _ = context.Injector().Invoke(handler.GetMethod())
	}
}

func (s *Server) handleGlobal(base string) []*RouteItem {
	result := make([]*RouteItem, 0)
	s.middleware.GetGlobal(func(mid IMiddlewareGlobal) bool {
		mid = mid.CloneAsCtl()
		r := mid.HandlerController(base)
		if r != nil {
			result = append(result, r)
		}

		return false
	})
	return result
}

func (s *Server) handleCtl(base string, ctl *astp.Struct) []*RouteItem {
	result := make([]*RouteItem, 0)
	attrs1 := attribute.GetStructAttrAsMiddleware(ctl)
	for _, attr := range attrs1 {
		if mid, ok := s.middleware.GetByAttributeCtl(attr.Name); ok {
			// 拷贝一份 表示这份实例唯此控制器独享
			mid = mid.CloneAsCtl()
			mid.SetParam(attr.Value)
			r := mid.HandlerController(base)
			if r != nil {
				result = append(result, r)
			}

		}
	}
	return result
}

func (s *Server) handle(handler *astp.Method, mids []IMiddlewareMethod) ([]string, HandlerFunc) {
	//先把实际的方法wrap成HandlerFunc
	next := s.wrapM(handler)
	// 先处理method上的中间件
	attrs := attribute.GetMethodAttributesAsMiddleware(handler)
	var attrs1 []string
	for _, attr := range attrs {
		if mid, ok := s.middleware.GetByAttributeMethod(attr.Name); ok {
			attrs1 = append(attrs1, mid.Attribute())
			// 拷贝一份副本 让中间件对于此上下文唯一
			mid = mid.CloneAsMethod()
			mid.SetParam(attr.Value)
			next = mid.HandlerMethod(next)
		}
	}
	// 然后处理controller上的中间件
	for _, mid := range mids {
		next = mid.HandlerMethod(next)
	}
	// 这里全局的中间件 仅针对于方法，不会对Controller做出改变
	for _, global := range s.midGlobals {
		next = global.HandlerMethod(next)
	}
	return attrs1, next
}

func (s *Server) handleError(ctx *Context, err error) {

}

func (s *Server) Run() error {
	s.tw.Render()

	internal.OKf("fw server@%s serving at http://%s:%d%s", Version, s.option.Listen, s.option.Port, s.option.BasePath)
	s.server.Handler = s.router.Handler
	s.server.StreamRequestBody = true
	s.server.Name = s.option.Name
	if s.option.Dev {
		if runtime.GOOS == "darwin" {
			internal.Note("press ⌘+C to exit...")
		} else {
			internal.Note("press CTRL+C to exit...")
		}
	}

	return s.server.ListenAndServe(fmt.Sprintf("%s:%d", s.option.Listen, s.option.Port))
}

// Use register middleware to server.
// you can only use the @'Attribute' after register a middleware
func (s *Server) Use(middleware IMiddleware) {
	s.middleware.Reg(middleware)
}
