package fw

import (
	"fmt"
	"github.com/fasthttp/router"
	"github.com/linxlib/astp"
	"github.com/linxlib/fw/attribute"
	"github.com/linxlib/fw/binding"
	"github.com/linxlib/fw/internal"
	"github.com/linxlib/fw/options"
	"github.com/linxlib/inject"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

const Version = "1.0.0-beta"

type HookHandler interface {
	HandleServerInfo(si []string)
	HandleStructs(ctl *astp.Element)
	HandleParams(pf *astp.Element)
}

func New() *Server {
	s := &Server{
		Injector:   inject.New(),
		router:     router.New(),
		server:     &fasthttp.Server{},
		option:     new(options.ServerOption),
		parser:     astp.NewParser(),
		middleware: NewMiddlewareContainer(),
		//logger:     logrus.StandardLogger(),
		routerTreeForPrint: make(map[string][][2]string),
	}
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgBlack)).WithFullWidth().Println("FW for golang developers")
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(Console())
	logger.SetLevel(logrus.InfoLevel)
	logger.SetReportCaller(true)

	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

	options.ReadConfig(s.option)
	logger.SetLevel(logrus.Level(s.option.Logger.LoggerLevel))
	dir := s.option.Logger.LogDir

	if !s.option.Logger.RotateFile {
		if s.option.Logger.SeparateLevelFile {

			pathMap := PathMap{
				logrus.InfoLevel:  dir + "/info.log",
				logrus.ErrorLevel: dir + "/error.log",
				logrus.DebugLevel: dir + "/debug.log",
			}
			logger.AddHook(NewFileHook(pathMap, Json()))
		} else {
			logger.AddHook(NewFileHook(dir+"/fw.log", Json()))
		}
	} else {
		if s.option.Logger.SeparateLevelFile {
			writerMap := WriterMap{
				logrus.InfoLevel: &lumberjack.Logger{
					Filename:   dir + "/info.log",
					Compress:   s.option.Logger.Compress,
					MaxSize:    s.option.Logger.MaxSize,
					MaxAge:     s.option.Logger.MaxAge,
					MaxBackups: s.option.Logger.MaxBackups,
					LocalTime:  s.option.Logger.LocalTime,
				},
				logrus.ErrorLevel: &lumberjack.Logger{
					Filename:   dir + "/error.log",
					Compress:   s.option.Logger.Compress,
					MaxSize:    s.option.Logger.MaxSize,
					MaxAge:     s.option.Logger.MaxAge,
					MaxBackups: s.option.Logger.MaxBackups,
					LocalTime:  s.option.Logger.LocalTime,
				},

				logrus.DebugLevel: &lumberjack.Logger{
					Filename:   dir + "/debug.log",
					Compress:   s.option.Logger.Compress,
					MaxSize:    s.option.Logger.MaxSize,
					MaxAge:     s.option.Logger.MaxAge,
					MaxBackups: s.option.Logger.MaxBackups,
					LocalTime:  s.option.Logger.LocalTime,
				},
			}
			logger.AddHook(NewFileHook(writerMap, Json()))
		} else {

			logger.AddHook(NewFileHook(&lumberjack.Logger{
				Filename:   dir + "/fw.log",
				Compress:   s.option.Logger.Compress,
				MaxSize:    s.option.Logger.MaxSize,
				MaxAge:     s.option.Logger.MaxAge,
				MaxBackups: s.option.Logger.MaxBackups,
				LocalTime:  s.option.Logger.LocalTime,
			}, Json()))
		}

	}

	s.logger = logger

	s.Map(s.option)
	s.Map(s.logger)
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
	s.tw.SetAutoMergeCellsByColumnIndex([]int{0})
	return s
}

type Server struct {
	inject.Injector
	server             *fasthttp.Server
	router             *router.Router
	option             *options.ServerOption
	parser             *astp.Parser
	tw                 *tablewriter.Table
	middleware         *MiddlewareContainer
	logger             *logrus.Logger
	once               sync.Once
	midGlobals         []IMiddlewareMethod
	hookHandler        HookHandler
	routerTreeForPrint map[string][][2]string
}

func (s *Server) RegisterHooks(handler HookHandler) {
	s.hookHandler = handler
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
	}
}

func (s *Server) addRouteTable(a, b, c, d, e string) {
	if v, ok := s.routerTreeForPrint[a]; ok {
		var temp [2]string
		temp[0] = e
		temp[1] = fmt.Sprintf("%s %s -> %s", b, c, d)
		v = append(v, temp)
		s.routerTreeForPrint[a] = v
	} else {
		s.routerTreeForPrint[a] = make([][2]string, 0)
		var temp [2]string
		temp[0] = e
		temp[1] = fmt.Sprintf("%s %s -> %s", b, c, d)
		s.routerTreeForPrint[a] = append(s.routerTreeForPrint[a], temp)
	}
	var fcolor = func(method string) []int {
		switch method {
		case "GET":
			return tablewriter.Color(tablewriter.FgBlueColor)
		case "POST":
			return tablewriter.Color(tablewriter.FgCyanColor)
		case "PUT":
			return tablewriter.Color(tablewriter.FgYellowColor)
		case "DELETE":
			return tablewriter.Color(tablewriter.FgRedColor)
		case "PATCH":
			return tablewriter.Color(tablewriter.FgGreenColor)
		case "HEAD":
			return tablewriter.Color(tablewriter.FgMagentaColor)
		case "OPTIONS":
			return tablewriter.Color(tablewriter.FgWhiteColor)
		case "WS":
			return tablewriter.Color(tablewriter.FgCyanColor)
		default:
			return tablewriter.Color(tablewriter.Normal)
		}
	}
	if s.option.NoColor {
		s.tw.Append([]string{a, b, c, d, e})
	} else {
		s.tw.Rich([]string{a, b, c, d, e}, []tablewriter.Colors{
			tablewriter.Color(tablewriter.FgBlueColor),
			fcolor(b),
			tablewriter.Color(tablewriter.Normal),
			tablewriter.Color(tablewriter.Normal),
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
	defer func() {
		if err := recover(); err != nil {
			s.logger.Error(err)
		}
	}()
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

				err := s.registerRoute(item.Method, item.Path, item.H)
				if err != nil {
					panic(err)
				}
				if !item.IsHide {
					s.addRouteTable("Global", item.Method, item.Path, "", "@"+item.Middleware.Name())
				}

			}
		}
	})

	// 遍历代码中所有的 @Controller 标记的结构，按照控制器对待
	s.parser.VisitStruct(func(element *astp.Element) bool {
		return element.Name == typ.Name() && attribute.HasAttribute(element, controllerAttr)
	}, func(ctl *astp.Element) {

		// 第一层路由 【配置文件】
		base := s.option.BasePath
		if base == "" {
			base = "/"
		}
		// 第二层路由 @Route 标记

		if r := attribute.GetStructAttrByName(ctl, controllerRoute); r != nil {
			base = joinRoute(base, r.Value)
		}
		ctl.SetRValue(refVal)
		ctl.SetValue(refVal.Interface())
		ctl.SetRType(typ)
		//处理控制器
		m := s.handleCtl(base, ctl)

		mids := make([]IMiddlewareMethod, 0)

		for _, item := range m {
			mids = append(mids, item.Middleware)
			if item.Path != "" {
				err := s.registerRoute(item.Method, item.Path, item.H)
				if err != nil {
					continue
				}
				if !item.IsHide {
					s.addRouteTable(ctl.Name, item.Method, item.Path, ctl.Name, "@"+item.Middleware.Name())
				}
			}

		}
		if s.hookHandler != nil {
			s.hookHandler.HandleStructs(ctl)
		}
		//处理控制器方法
		ctl.VisitElements(astp.ElementMethod, func(element *astp.Element) bool {
			return !element.Private()
		}, func(method *astp.Element) {
			vm := refVal.MethodByName(method.Name)
			vmt := reflect.TypeOf(vm.Interface())

			method.VisitElementsAll(astp.ElementParam, func(param *astp.Element) {
				param.SetRType(vmt.In(param.Index))
			})
			method.SetRValue(vm)
			method.SetValue(vm.Interface())
			var hms = make([]string, 0)
			var rps = make([]string, 0)
			var toIgnore string
			for _, command := range attribute.GetMethodAttributes(method) {
				if command.Type == attribute.TypeHttpMethod {
					hms = append(hms, command.Name)
					rps = append(rps, command.Value)
				} else if command.Name == "Ignore" && command.Value != "" {
					//处理忽略
					toIgnore = command.Value
				}
			}
			attrs, call1 := s.handle(method, mids, toIgnore)
			sig := strings.Builder{}
			for _, mid := range mids {
				sig.WriteString("@")
				sig.WriteString(mid.Attribute())
				sig.WriteRune(',')
			}
			for _, attr := range attrs {
				sig.WriteString("@")
				sig.WriteString(attr)
				sig.WriteRune(',')
			}

			for i, hm := range hms {
				err := s.registerRoute(strings.ToUpper(hm), joinRoute(base, rps[i]), call1)
				if err != nil {
					continue
				}
				receiver := method.MustGetElement(astp.ElementReceiver)
				if receiver != nil {
					controllerName := receiver.TypeString
					route := joinRoute(base, rps[i])
					if receiver.TypeString != ctl.Name {
						controllerName = ctl.Name
						sig.WriteString("@inherit")
					}
					s.addRouteTable(controllerName, strings.ToUpper(hm), route, method.Name, sig.String())
				}

			}

		})

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

func (s *Server) bind(c *Context, handler *astp.Element) error {
	// 准备调用方法需要的参数值
	params := handler.ElementsAll(astp.ElementParam)
	for _, param := range params {
		if param.GetRType().AssignableTo(reflect.TypeOf(c)) {
			c.Map(c)
			continue
		}

		//TODO: 根据请求方法和contentType进行binding
		cmd := attribute.GetLastAttr(param)
		if cmd.Type == attribute.TypeInner {

		} else {
			//TODO: 是否要兼容 非指针方式声明的参数
			body := reflect.New(param.GetRType().Elem())
			if strings.ToLower(cmd.Name) == "service" || cmd.Name == "" {

			} else {
				if s.hookHandler != nil {
					s.hookHandler.HandleParams(param)
				}

				// 对方法参数进行数据映射和校验
				if err := binding.GetByAttr(cmd).Bind(c.GetFastContext(), body.Interface()); err != nil {
					return err
				}
			}
			c.Map(body.Interface())
		}
	}

	return nil
}

func (s *Server) wrapM(handler *astp.Element) HandlerFunc {
	return func(context *Context) {
		var err error
		err = s.bind(context, handler)
		if err != nil {
			panic(err)
		}
		_, err = context.Invoke(handler.GetValue())
		if err != nil {
			panic(err)
		}
	}
}

func (s *Server) handleGlobal(base string) []*RouteItem {
	result := make([]*RouteItem, 0)
	s.middleware.GetGlobal(func(mid IMiddlewareGlobal) bool {
		mid = mid.CloneAsCtl()
		r := mid.HandlerController(base)
		if r != nil {
			result = append(result, r...)
		}

		return false
	})
	return result
}

func (s *Server) handleCtl(base string, ctl *astp.Element) []*RouteItem {
	result := make([]*RouteItem, 0)
	attrs1 := attribute.GetStructAttrAsMiddleware(ctl)
	for _, attr := range attrs1 {
		if mid, ok := s.middleware.GetByAttributeCtl(attr.Name); ok {
			// 拷贝一份 表示这份实例唯此控制器独享
			mid = mid.CloneAsCtl()
			mid.SetParam(attr.Value)
			mid.SetCtlRValue(ctl.GetRValue())
			r := mid.HandlerController(base)
			if r != nil {
				result = append(result, r...)
			}

		}
	}
	return result
}

func (s *Server) handle(handler *astp.Element, mids []IMiddlewareMethod, toIgnore string) ([]string, HandlerFunc) {
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
			mid.SetMethodRValue(handler.GetRValue())
			next = mid.HandlerMethod(next)
		}
	}
	// 然后处理controller上的中间件
	for _, mid := range mids {
		mid.SetMethodRValue(handler.GetRValue())
		// 如果方法上打了 @Ignore Auth 则需要忽略 Auth这个代表 AuthMiddleware 的中间件
		if toIgnore == mid.Attribute() {
			next = mid.HandlerIgnored(next)
			continue
		}
		next = mid.HandlerMethod(next)
	}
	// 这里全局的中间件 仅针对于方法，不会对Controller做出改变
	for _, global := range s.midGlobals {
		next = global.HandlerMethod(next)
	}
	return attrs1, next
}

func (s *Server) Run() error {
	s.tw.Render()
	var node pterm.TreeNode = pterm.TreeNode{
		Text: "Server",
	}

	for s2, i := range s.routerTreeForPrint {
		no := pterm.TreeNode{
			Text: s2,
		}
		for _, i3 := range i {
			no.Children = append(no.Children,
				pterm.TreeNode{
					Text: i3[1] + " " + i3[0],
				})
		}
		node.Children = append(node.Children, no)
	}
	pterm.DefaultTree.WithRoot(node).Render()

	//s.logger.Printf("fw server@%s serving at http://%s:%d%s", Version, s.option.Listen, s.option.Port, s.option.BasePath)
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
	err := s.Apply(middleware)
	if err != nil {
		log.Println(err)
	}
	s.middleware.Reg(middleware)
}
