package fw

import (
	"fmt"
	"github.com/fasthttp/router"
	"github.com/gookit/color"
	"github.com/linxlib/astp"
	"github.com/linxlib/config"
	"github.com/linxlib/fw/attribute"
	"github.com/linxlib/fw/binding"
	"github.com/linxlib/fw/internal"
	"github.com/linxlib/fw/types"
	"github.com/linxlib/inject"
	"github.com/pterm/pterm"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

const Version = "v1.0.0@beta"

type HookHandler interface {
	HandleServerInfo(si []string)
	HandleStructs(ctl *astp.Element)

	Print(slot string)
}

type ServerOption struct {
	IntranetIP            string
	Dev                   bool         `yaml:"dev" default:"true"`
	Debug                 bool         `yaml:"debug" default:"true"`
	NoColor               bool         `yaml:"nocolor" default:"false"`
	BasePath              string       `yaml:"basePath" default:"/"`
	Listen                string       `yaml:"listen" default:"127.0.0.1"` //监听地址
	Title                 string       `yaml:"title" default:"fw api"`
	Name                  string       `yaml:"name" default:"fw"` //server_token
	ShowRequestTimeHeader bool         `yaml:"showRequestTimeHeader,omitempty" default:"true"`
	RequestTimeHeader     string       `yaml:"requestTimeHeader,omitempty" default:"Request-Time"`
	Port                  int          `yaml:"port" default:"2024"`
	AstFile               string       `yaml:"astFile" default:"gen.json"` //ast json file generated by github.com/linxlib/astp. default is gen.json
	Logger                LoggerOption `yaml:"logger"`
}
type LoggerOption struct {
	LoggerLevel       int    `yaml:"loggerLevel" default:"4"` //0-6 0: Panic 6: Trace
	SeparateLevelFile bool   `yaml:"separateLevelFile" default:"false"`
	LogDir            string `yaml:"logDir" default:"log"`
	RotateFile        bool   `yaml:"rotate" default:"true"`
	MaxSize           int    `yaml:"maxSize" default:"5"`
	MaxAge            int    `yaml:"maxAge" default:"28"`
	MaxBackups        int    `yaml:"maxBackups" default:"3"`
	Compress          bool   `yaml:"compress" default:"false"`
	LocalTime         bool   `yaml:"localTime" default:"true"`
}

func New(key ...string) *Server {
	s := &Server{
		Injector:           inject.New(),
		router:             router.New(),
		server:             &fasthttp.Server{},
		option:             new(ServerOption),
		parser:             astp.NewParser(),
		middleware:         NewMiddlewareContainer(),
		routerTreeForPrint: make(map[string][][2]string),
		beginTime:          time.Now(),
		plugins:            make([]IPlugin, 0),
	}
	s.conf = config.New(&config.Option{
		AutoReload:         true,
		Silent:             true,
		ENVPrefix:          "FW",
		Files:              []string{"config/config.yaml"},
		AutoReloadInterval: time.Second * 1,
		AutoReloadCallback: func(key string, config interface{}) {
			fmt.Println("key=", key, "config=", config)
		},
	})
	s.option.IntranetIP = getIntranetIP()
	configKey := ""
	if len(key) > 0 {
		configKey = key[0]
	}
	err := s.conf.LoadWithKey(configKey, s.option)
	if err != nil {
		panic(err)
	}
	//pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgBlack)).WithFullWidth().Println("FW for golang developers")

	s.configLogger()

	s.Map(s.option)
	s.Map(s.conf)

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
	return s
}

type Server struct {
	inject.Injector
	server             *fasthttp.Server
	router             *router.Router
	option             *ServerOption
	conf               *config.Config
	parser             *astp.Parser
	middleware         *MiddlewareContainer
	logger             *logrus.Logger
	once               sync.Once
	midGlobals         []IMiddlewareCtl
	routerTreeForPrint map[string][][2]string
	beginTime          time.Time
	plugins            []IPlugin
}

type IPlugin interface {
	InitPlugin(s *Server)
	HandleServerInfo(si []string)
	HandleStructs(ctl *astp.Element)
	Print(slot string)
}

func (s *Server) AddPlugin(plugin IPlugin) {
	s.plugins = append(s.plugins, plugin)
}

func (s *Server) configLogger() {
	var l types.ILogger = new(types.Logger)
	s.Map(l)

	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(Console())
	logger.SetLevel(logrus.InfoLevel)
	logger.SetReportCaller(true)

	logger.SetLevel(logrus.Level(s.option.Logger.LoggerLevel))
	dir := s.option.Logger.LogDir

	if !s.option.Logger.RotateFile {
		if s.option.Logger.SeparateLevelFile {

			pathMap := PathMap{
				logrus.InfoLevel:  dir + "/info.log",
				logrus.ErrorLevel: dir + "/error.log",
				logrus.DebugLevel: dir + "/debug.log",
			}
			logger.AddHook(NewFileHook(pathMap, File()))
		} else {
			logger.AddHook(NewFileHook(dir+"/fw.log", File()))
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
			logger.AddHook(NewFileHook(writerMap, File()))
		} else {

			logger.AddHook(NewFileHook(&lumberjack.Logger{
				Filename:   dir + "/fw.log",
				Compress:   s.option.Logger.Compress,
				MaxSize:    s.option.Logger.MaxSize,
				MaxAge:     s.option.Logger.MaxAge,
				MaxBackups: s.option.Logger.MaxBackups,
				LocalTime:  s.option.Logger.LocalTime,
			}, File()))
		}

	}

	s.logger = logger
	s.Map(s.logger)
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

func (s *Server) RegisterRoutes(controller ...any) {
	for _, a := range controller {
		s.RegisterRoute(a)
	}
}

const controllerAttr = "Controller"
const controllerRoute = "Route"

func (s *Server) RegisterRoute(controller any) {

	if v, ok := controller.(IController); ok {
		v.Init(s)
	}
	if v, ok := controller.(IControllerConfig); ok {
		v.InitConfig(s.conf)
	}
	//defer func() {
	//	if err := recover(); err != nil {
	//		s.logger.Error(err)
	//	}
	//}()
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
		for _, plugin := range s.plugins {
			plugin.InitPlugin(s)
		}
		s.midGlobals = make([]IMiddlewareCtl, 0)
		routeItems := make([]*RouteItem, 0)
		s.middleware.GetGlobal(func(mid IMiddlewareGlobal) bool {
			ctx := newMiddlewareContext(mid.Name(), "", SlotGlobal, "", nil)
			r := mid.Router(ctx)
			if r != nil {
				routeItems = append(routeItems, r...)
			}
			s.midGlobals = append(s.midGlobals, mid)
			return false
		})
		for _, item := range routeItems {
			if item.Path != "" && item.Method != "" {
				err := s.registerRoute(item.Method, joinRoute(s.option.BasePath, item.Path, item.OverrideBasePath), item.H)
				if err != nil {
					panic(err)
				}
				if !item.IsHide {
					s.addRouteTable("Global", item.Method, joinRoute(s.option.BasePath, item.Path, item.OverrideBasePath), item.Middleware.Name()+".H", "@"+item.Middleware.Name())
				}
			}
		}

	})

	// 遍历代码中所有的 @Controller 标记的结构，按照控制器对待
	s.parser.VisitStruct(func(element *astp.Element) bool {
		return element.Name == typ.Name() && (attribute.HasAttribute(element, controllerAttr) || strings.HasSuffix(element.Name, controllerAttr))
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
		middlewareCtls := make([]IMiddlewareCtl, 0)
		routeItems := make([]*RouteItem, 0)
		attrs1 := attribute.GetStructAttrAsMiddleware(ctl)
		for _, attr := range attrs1 {
			if mid, ok := s.middleware.GetByAttributeCtl(attr.Name); ok {
				ctx := newMiddlewareContext(ctl.Name, "", SlotController, attr.Value, nil)
				ctx.SetRValue(ctl.GetRValue())
				r := mid.Router(ctx)
				if r != nil {
					routeItems = append(routeItems, r...)
				}
				middlewareCtls = append(middlewareCtls, mid)

			}
		}

		for _, item := range routeItems {
			if item.Path != "" && item.Method != "" {
				err := s.registerRoute(item.Method, joinRoute(base, item.Path, item.OverrideBasePath), item.H)
				if err != nil {
					continue
				}
				if !item.IsHide {
					s.addRouteTable(ctl.Name, item.Method, joinRoute(base, item.Path, item.OverrideBasePath), ctl.Name, "@"+item.Middleware.Attribute())
				}
			}
		}
		for _, plugin := range s.plugins {
			plugin.HandleStructs(ctl)
		}
		//处理控制器方法
		ctl.VisitElements(astp.ElementMethod, func(element *astp.Element) bool {
			return !element.Private()
		}, func(method *astp.Element) {
			vm := refVal.MethodByName(method.Name)
			vmt := reflect.TypeOf(vm.Interface())
			// 此处将方法参数得反射类型（reflect.Type）暂存
			method.VisitElementsAll(astp.ElementParam, func(param *astp.Element) {
				param.SetRType(vmt.In(param.Index))
			})
			// 方法的reflect.Value暂存，用于传递给中间件
			method.SetRValue(vm)
			method.SetValue(vm.Interface())
			var hms = make([]string, 0)
			var rps = make([]string, 0)
			var toIgnore string
			for _, attr := range attribute.GetMethodAttributes(method) {
				if attr.Type == attribute.TypeHttpMethod {
					hms = append(hms, attr.Name)
					rps = append(rps, attr.Value)
				} else if strings.ToUpper(attr.Name) == "IGNORE" && attr.Value != "" {
					//处理忽略
					toIgnore = strings.ToUpper(attr.Value)
				}
			}
			// 先处理方法上标记的中间件
			attrs, next := s.handle(ctl, method)
			// 然后处理controller上的中间件
			for _, mid := range middlewareCtls {

				ctx := newMiddlewareContext(ctl.Name, method.Name, SlotMethod, "", next)
				ctx.SetRValue(ctl.GetRValue())
				// 如果方法上打了 @Ignore Auth 则需要忽略 Auth这个代表 AuthMiddleware 的中间件
				//TODO: 是否需要处理 @Ignore 多个的情况？
				if toIgnore == strings.ToUpper(mid.Attribute()) {
					ctx.Ignored = true
				} else {
					attrs = append(attrs, mid.Attribute())
				}
				next = mid.Execute(ctx)
			}
			// 这里全局的中间件 仅针对于方法，不会对Controller做出改变
			for _, global := range s.midGlobals {
				ctx := newMiddlewareContext(global.Name(), "", SlotGlobal, "", next)
				next = global.Execute(ctx)
			}

			sig := strings.Builder{}
			for i, attr := range attrs {
				if i != 0 {
					sig.WriteRune(',')
				}
				sig.WriteString("@")
				sig.WriteString(attr)
			}

			for i, hm := range hms {
				err := s.registerRoute(strings.ToUpper(hm), joinRoute(base, rps[i]), next)
				if err != nil {
					continue
				}
				receiver := method.MustGetElement(astp.ElementReceiver)
				controllerName := receiver.TypeString
				route := joinRoute(base, rps[i])
				if method.FromParent {
					if sig.Len() != 0 {
						sig.WriteRune(',')
					}

					sig.WriteString("@inherit")
				}
				s.addRouteTable(controllerName, strings.ToUpper(hm), route, method.Name, sig.String())
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
		cmd := new(attribute.Attribute)
		//  fix for generic
		if param.ItemType == astp.ElementGeneric {
			if param.Item != nil && (param.Item.ElementType == astp.ElementGeneric || param.Item.ItemType == astp.ElementGeneric) {
				cmd = attribute.GetLastAttr(param.Item)
			}
		} else {
			cmd = attribute.GetLastAttr(param)
		}

		//TODO: 根据请求方法和contentType进行binding

		if (c.Method() == "GET" || c.Method() == "HEAD") && (strings.ToLower(cmd.Name) == "body" || strings.ToLower(cmd.Name) == "json") {
			//类似不合规的参数, 进行跳过
			continue
		}

		if cmd.Type == attribute.TypeInner {
			continue
		}
		//TODO: 是否要兼容 非指针方式声明的参数
		paramV := reflect.New(param.GetRType().Elem())
		if strings.ToLower(cmd.Name) == "service" {

		} else {

			// 对方法参数进行数据映射和校验
			binder := binding.GetByAttr(cmd)
			if binding.IsBodyBinder(binder) {
				if c.Method() == "POST" || c.Method() == "PUT" || c.Method() == "PUT" || c.Method() == "PATCH" || c.Method() == "DELETE" {
					switch c.GetHeader("Content-Type") {
					case "application/json":
						binder = binding.JSON
					case "application/xml":
						binder = binding.XML
					case "application/x-www-form-urlencoded":
						binder = binding.Form
					case "multipart/form-data":
						binder = binding.FormMultipart
					case "text/plain":
						binder = binding.Plain

					}
				}
			}

			if err := binder.Bind(c.GetFastContext(), paramV.Interface()); err != nil {
				return err
			}
		}
		c.Map(paramV.Interface())

	}

	return nil
}

func (s *Server) wrapM(handler *astp.Element) HandlerFunc {
	return func(context *Context) {
		defer func() {
			if err := recover(); err != nil {
				if err != "fw" {
					panic(err)
				}
			}
		}()
		var err error
		// binding params
		err = s.bind(context, handler)
		if err != nil {
			panic(err)
		}
		// call method
		values, err := context.Invoke(handler.GetValue())
		if err != nil {
			panic(err)
		}
		last := len(values) - 1
		if last == -1 { // if there is no return value, just skip.
			if !context.hasReturn {
				context.SendStatus(200)
			}
			return
		}

		if err := values[last]; !err.IsZero() {
			// if the last return value is error, parse it and write error info into response body
			if e, ok := err.Interface().(error); ok {
				context.ErrorExit(e)
			} else { // If there is no error return value, the return value will be treated as a normal return.
				// and only one return value will be written into response body
				if !context.hasReturn {
					context.PureJSON(200, values[0].Interface())
				}
			}
		} else {
			// method returns error, just ignore others.
			if !context.hasReturn {
				if values[0].IsNil() {
					context.Status(200)
				} else {
					context.PureJSON(200, values[0].Interface())
				}

			}
			//context.Error(err.Interface().(error))
		}
	}
}

func (s *Server) handle(ctl *astp.Element, handler *astp.Element) ([]string, HandlerFunc) {
	//先把实际的方法wrap成HandlerFunc
	next := s.wrapM(handler)
	// 先处理method上的中间件
	attrs := attribute.GetMethodAttributesAsMiddleware(handler)
	var attrs1 []string
	for _, attr := range attrs {
		if mid, ok := s.middleware.GetByAttributeMethod(attr.Name); ok {
			attrs1 = append(attrs1, mid.Attribute())

			ctx := newMiddlewareContext(ctl.Name, handler.Name, SlotMethod, attr.Value, next)
			ctx.SetRValue(ctl.GetRValue())
			next = mid.Execute(ctx)
		}
	}
	return attrs1, next
}

func (s *Server) addRouteTable(controllerName, method, routePath, methodName, signature string) {
	var fcolor1 = func(method string) string {
		switch method {
		case "GET":
			return color.Blue.Sprint(method)
		case "POST":
			return color.Cyan.Sprint(method)
		case "PUT":
			return color.Yellow.Sprint(method)
		case "DELETE":
			return color.Red.Sprint(method)
		case "PATCH":
			return color.Green.Sprint(method)
		case "HEAD":
			return color.Magenta.Sprint(method)
		case "OPTIONS":
			return color.White.Sprint(method)
		default:
			return color.Normal.Sprint(method)
		}
	}
	const itemFmt = "%-16s %-30s%-30s"
	controllerName = color.Magenta.Sprint(controllerName)
	if v, ok := s.routerTreeForPrint[controllerName]; ok {
		var temp [2]string
		temp[0] = color.HiYellow.Sprint(signature)
		if methodName == "" {
			temp[1] = fmt.Sprintf(itemFmt, fcolor1(method), routePath, color.HiGreen.Sprint(""))
		} else {
			temp[1] = fmt.Sprintf(itemFmt, fcolor1(method), routePath, "-> "+color.HiGreen.Sprint(methodName))
		}

		v = append(v, temp)
		s.routerTreeForPrint[controllerName] = v
	} else {
		s.routerTreeForPrint[controllerName] = make([][2]string, 0)
		var temp [2]string
		temp[0] = color.HiYellow.Sprint(signature)
		if methodName == "" {
			temp[1] = fmt.Sprintf(itemFmt, fcolor1(method), routePath, color.HiGreen.Sprint(""))
		} else {
			temp[1] = fmt.Sprintf(itemFmt, fcolor1(method), routePath, "-> "+color.HiGreen.Sprint(methodName))
		}
		s.routerTreeForPrint[controllerName] = append(s.routerTreeForPrint[controllerName], temp)
	}
}

func (s *Server) printRoute() {
	var node = pterm.TreeNode{
		Text: "FW Server",
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
	_ = pterm.DefaultTree.WithRoot(node).Render()
}

func (s *Server) printInfo() {
	style := pterm.NewStyle(pterm.FgLightGreen, pterm.Bold)
	style1 := pterm.NewStyle(pterm.FgLightGreen)
	style2 := pterm.NewStyle(pterm.FgDarkGray)
	style3 := pterm.NewStyle(pterm.FgLightWhite, pterm.Bold)
	style4 := pterm.NewStyle(pterm.FgWhite)
	style.Print("FW ")
	style1.Print(Version + " ")
	style2.Print("ready in ")
	style3.Println(time.Now().Sub(s.beginTime).String())

	//color.Printf("%s %s %s\n", color.HiGreen.Sprintf("FW %s", Version), color.Gray.Sprint("ready in"), color.HiWhite.Sprint("568ms"))
	style.Print("  ➜ ")
	style3.Printf("%10s", "Local: ")
	style4.Printf("http://%s:%d%s\n", "localhost", s.option.Port, s.option.BasePath)
	if s.CanAccessByLan() {
		style.Print("  ➜ ")
		style3.Printf("%10s", "Network: ")
		style4.Printf("http://%s:%d%s\n", s.option.IntranetIP, s.option.Port, s.option.BasePath)
	}
	for _, plugin := range s.plugins {
		plugin.Print(AfterListen)
	}
	if s.option.Dev {
		if runtime.GOOS == "darwin" {
			internal.Note("press ⌘+C to exit...")
		} else {
			internal.Note("press CTRL+C to exit...")
		}
	}
}
func (s *Server) CanAccessByLan() bool {
	if strings.EqualFold(s.option.IntranetIP, s.option.Listen) || strings.EqualFold(s.option.Listen, "0.0.0.0") {
		return true
	}
	return false
}

func (s *Server) Run() chan bool {
	return s.start()
}
func (s *Server) start() chan bool {

	for _, plugin := range s.plugins {
		for _, file := range s.parser.Files {
			if !file.IsMain() {
				continue
			}
			plugin.HandleServerInfo(file.Comments)
		}
	}

	s.printRoute()

	s.server.Handler = s.router.Handler
	s.server.StreamRequestBody = true
	s.server.Name = s.option.Name

	done := make(chan bool)
	go func() {
		err := s.server.ListenAndServe(fmt.Sprintf("%s:%d", s.option.Listen, s.option.Port))
		if err != nil {
			internal.Errorf("Failed to start server: %v", err)
			done <- true
			return
		}
	}()

	s.printInfo()
	return done
}
func (s *Server) ListenAddr() string {
	return s.option.Listen
}
func (s *Server) Port() int {
	return s.option.Port
}
func (s *Server) Schema() string {
	return "http"
}
func (s *Server) BasePath() string {
	return strings.TrimSuffix(s.option.BasePath, "/")
}

func (s *Server) Start() {
	done := s.start()
	for {
		select {
		case <-done:
			return
		}
	}
}

const (
	AfterListen = "afterListen"
)

// Use register middleware to server.
// you can only use the @'Attribute' after register a middleware
func (s *Server) Use(middleware ...IMiddleware) {
	if len(middleware) <= 0 {
		return
	}
	for _, iMiddleware := range middleware {
		iMiddleware.setConfig(s.conf)
		iMiddleware.setProvider(s)
		_ = s.Apply(iMiddleware)
		iMiddleware.DoInitOnce()
		s.middleware.Reg(iMiddleware)
	}

}

func (s *Server) UseMapper(mapper ...ServiceMapper) {
	if len(mapper) <= 0 {
		return
	}
	for _, serviceMapper := range mapper {
		result, err := serviceMapper.Init(s.conf)
		if err != nil {
			panic(err)
		}
		s.Map(result)
	}

}

func (s *Server) UseService(service ...IService) {
	if len(service) <= 0 {
		return
	}
	for _, iService := range service {
		iService.Init(s)
		s.Map(iService)
	}

}

func (s *Server) UseServiceWithConfig(service ...IServiceConfig) {
	if len(service) <= 0 {
		return
	}
	for _, serviceConfig := range service {
		serviceConfig.InitConfig(s.conf)
		s.Map(serviceConfig)
	}

}
