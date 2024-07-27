package fw

import (
	"fmt"
	"github.com/fasthttp/router"
	"github.com/linxlib/astp"
	"github.com/linxlib/fw/attribute"
	"github.com/linxlib/fw/binding"
	"github.com/linxlib/fw/internal"
	"github.com/linxlib/fw/types"
	"github.com/linxlib/inject"
	"github.com/olekukonko/tablewriter"
	"github.com/valyala/fasthttp"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
)

const Version = "1.0.0-beta"

func New(f ...func(*Option)) *Server {
	s := &Server{
		Injector:   inject.New(),
		router:     router.New(),
		server:     &fasthttp.Server{},
		option:     defaultOption(),
		parser:     astp.NewParser(), //0 非运行时 1 运行时
		middleware: NewMiddlewareContainer(),
		logger:     &Logger{},
	}
	if len(f) > 0 {
		f[0](s.option)
	}
	if os.Getenv("FW_DEBUG") == "true" {
		s.isDev = true
	}
	s.Map(s.option.y)
	s.Map(s)
	s.parser.Load()
	s.tw = tablewriter.NewWriter(os.Stdout)
	s.tw.SetHeader([]string{"Controller", "Method", "Route", "Method", "Signature"})
	s.tw.SetRowLine(true)
	s.tw.SetCenterSeparator("|")
	writeConfig(s.option)
	return s
}

type Server struct {
	inject.Injector
	server     *fasthttp.Server
	router     *router.Router
	option     *Option
	parser     *astp.Parser
	tw         *tablewriter.Table
	middleware *MiddlewareContainer
	logger     types.ILogger
	once       sync.Once
	midGlobals []IMiddlewareMethod
	isDev      bool
}

type HandlerFunc = func(*Context)

// wrap the HandlerFunc to fasthttp.RequestHandler
// just create *Context
func (s *Server) wrap(h HandlerFunc) fasthttp.RequestHandler {

	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()
		c := newContext(ctx, s)
		h(c)
		if s.option.Server.ShowRequestTimeHeader {
			c.ctx.Response.Header.Set("Request-Time", time.Since(start).String())
		}
		fmt.Printf("call spend: %s\n", time.Since(start).String())
	}
}

func (s *Server) handleStatic() {
	if len(s.option.Server.StaticDirs) > 0 {
		for _, sd := range s.option.Server.StaticDirs {
			var path string
			if strings.HasPrefix(sd.Path, "/") {
				path = sd.Path + "/{filepath:*}"
			} else {
				path = "/" + sd.Path + "/{filepath:*}"
			}
			s.router.ServeFiles(path, sd.Root)

			s.add("Server", "GET", path, sd.Root, "Files")
		}

	}
}

func (s *Server) add(a, b, c, d, e string) {
	s.tw.Append([]string{a, b, c, d, e})
}

// TODO: 直接注册一个function
func (s *Server) RegisterFunc(f any) error {
	return nil
}

func (s *Server) RegisterRoutes(controller ...any) {
	for _, a := range controller {
		s.RegisterRoute(a)
	}
}

func (s *Server) RegisterRoute(controller any) error {
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
		m0 := s.handleGlobal(s.option.Server.BasePath)
		for _, item := range m0 {
			s.midGlobals = append(s.midGlobals, item.Middleware)
			if item.Path != "" {
				s.registerRoute(item.Method, item.Path, item.H)
				if !item.IsHide {
					s.add("Server", item.Method, item.Path, "Global", "@"+item.Middleware.Name())
				}

			}

			//if !item.IsHide {
			//
			//}

		}
	})

	// 遍历代码中所有的 @Controller 标记的结构，按照控制器对待
	s.parser.VisitAllStructs(typ.Name(), func(ctl *astp.Struct) bool {
		if !attribute.HasAttribute(ctl, "Controller") {
			return false
		}
		// 第一层路由 【配置文件】
		base := s.option.Server.BasePath
		// 第二层路由 @Route 标记
		if r := attribute.GetStructAttrByName(ctl, "Route"); r != nil {
			base += r.Value
		}

		//处理控制器
		m := s.handleCtl(base, ctl)
		mids := make([]IMiddlewareMethod, 0)

		for _, item := range m {
			mids = append(mids, item.Middleware)
			if item.Path != "" {
				s.registerRoute(item.Method, item.Path, item.H)
				if !item.IsHide {
					s.add(ctl.Name, item.Method, item.Path, ctl.Name, "@"+item.Middleware.Name())
				}
			}

		}

		//处理控制器方法
		for _, method := range ctl.Methods {
			vm := refVal.MethodByName(method.Name)
			vmt := reflect.TypeOf(vm.Interface())
			//TODO: 这里获得到反射内容后 也许不需要存储到对象中
			//TODO: param应该自带参数的索引 因为循环是不确定的
			for i, param := range method.Params {
				param.SetRType(vmt.In(i))
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
				err := s.registerRoute(strings.ToUpper(hm), base+rps[i], call1)
				if err != nil {
					s.handleError(nil, err)
					continue
				}
				s.add(method.Receiver.TypeString, strings.ToUpper(hm), base+rps[i], method.Name, strings.Join(sig, ","))
			}

		}
		return false
	})

	return nil

}

func (s *Server) registerRoute(httpmethod string, relativePath string, call HandlerFunc) error {
	call1 := s.wrap(call)

	switch httpmethod {
	case "POST":
		s.router.POST(relativePath, call1)
	case "GET":
		s.router.GET(relativePath, call1)
	case "DELETE":
		s.router.DELETE(relativePath, call1)
	case "PATCH":
		s.router.PATCH(relativePath, call1)
	case "PUT":
		s.router.PUT(relativePath, call1)
	case "OPTIONS":
		s.router.OPTIONS(relativePath, call1)
	case "HEAD":
		s.router.HEAD(relativePath, call1)
	case "ANY", "WS":
		s.router.ANY(relativePath, call1)
	default:
		return fmt.Errorf("http method:[%v -> %s] not supported", httpmethod, relativePath)
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
		//log.Println("wrapM called")
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
	//log.Println("handle called")
	//先把实际的方法wrap成HandlerFunc
	next := s.wrapM(handler)
	// 先处理method上的中间件
	attrs := attribute.GetMethodAttributesAsMiddleware(handler)
	attrs1 := []string{}
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

func AddCtlAttributeType(name string, t attribute.AttributeType) {
	attribute.AddStructAttributeType(name, t)
}
func AddMethodAttributeType(name string, typ attribute.AttributeType) {
	attribute.AddMethodAttributeType(name, typ)
}

func (s *Server) handleError(ctx *Context, err error) {

}

func (s *Server) Run() error {
	s.handleStatic()
	s.tw.Render()

	internal.Infof("fw server running http://%s:%d%s\n", s.option.Server.Listen, s.option.Server.Port, s.option.Server.BasePath)
	s.server.Handler = s.router.Handler
	s.server.StreamRequestBody = true
	s.server.Name = "fw"
	return s.server.ListenAndServe(fmt.Sprintf("%s:%d", s.option.Server.Listen, s.option.Server.Port))
}

// Use register middleware to server.
// you can only use the @'Attribute' after register a middleware
func (s *Server) Use(middleware IMiddleware) {
	s.middleware.Reg(middleware)
}
