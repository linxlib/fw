package fw

import (
	"github.com/linxlib/config"
	"github.com/linxlib/fw/attribute"
	"net/url"
	"reflect"
	"strings"
)

type AttributeName = string
type SlotType = string

// MiddlewareContext represents the context in which a middleware is executed.
type MiddlewareContext struct {
	ControllerName string
	MethodName     string
	Location       SlotType
	param          map[SlotType]string
	rValue         map[SlotType]reflect.Value
	paramValues    url.Values
	rawParams      string
	Ignored        bool
	Next           HandlerFunc
}

// VisitParams calls the given function f for each key/value pair in
// middleware context's parameters.
//
// f is called with the key and value of each parameter. The value is a slice
// of strings.
func (m *MiddlewareContext) VisitParams(f func(key string, value []string)) {
	// Iterate over all key/value pairs in the parameters.
	for s, i := range m.paramValues {
		f(s, i)
	}
}

// DelParam deletes the value associated with key from middleware context's
// parameters.
func (m *MiddlewareContext) DelParam(key string) {
	m.paramValues.Del(key)
}

func (m *MiddlewareContext) SetRValue(v reflect.Value) {
	m.rValue[m.Location] = v
}

func (m *MiddlewareContext) GetRValue() reflect.Value {
	if ss, ok := m.rValue[m.Location]; ok {
		return ss
	} else {
		return reflect.Value{}
	}
}

// GetRawParams returns the raw parameters of the middleware context.
func (m *MiddlewareContext) GetRawParams() string {
	return m.rawParams
}

// newMiddlewareContext creates a new MiddlewareContext instance.
//
// ctlName is the name of the controller, methodName is the name of the method,
// location is the location of the middleware, param is the parameter string,
// and next is the next handler function in the chain.
// Returns a pointer to a MiddlewareContext instance.
func newMiddlewareContext(ctlName, methodName string, location SlotType, param string, next HandlerFunc) *MiddlewareContext {
	m := &MiddlewareContext{ControllerName: ctlName, MethodName: methodName, Location: location, Next: next}
	m.param = make(map[SlotType]string)
	m.rValue = make(map[SlotType]reflect.Value)
	m.param[location] = param
	m.rawParams = strings.TrimSpace(param)
	var err error
	m.paramValues, err = url.ParseQuery(param)
	if err != nil {
		m.paramValues = make(url.Values)
	}
	return m
}

// GetParam returns the value associated with key from middleware context's
// parameters.
//
// If the key is present in the map, the value (string) is returned.
// If the key is not present in the map, an empty string is returned.
func (m *MiddlewareContext) GetParam(key string) string {
	if ss, ok := m.paramValues[key]; ok {
		return strings.Join(ss, ",")
	} else {
		return ""
	}
}

type IMiddlewareBase interface {
	// Name returns the middleware's name
	Name() string
	// Attribute returns the middleware's Attribute just like Websocket so that you can use it like // @Websocket
	Attribute() AttributeName
	// GetSlot returns slot type
	GetSlot() SlotType
}

// IInitOnce is an interface that will be called only once
type IInitOnce interface {
	DoInitOnce()
}
type IConfig interface {
	setConfig(conf *config.Config)
	setProvider(provider IProvider)
	LoadConfig(key string, config any)
}
type IReg interface {
	// doReg inner called by fw middleware container
	doReg()
}

// IMiddleware
// interface of middleware
type IMiddleware interface {
	IMiddlewareBase
	IConfig
	IInitOnce
	IReg
	IProvider
}

var _ IMiddleware = (*Middleware)(nil)

// NewMiddleware creates a new Middleware instance
func NewMiddleware(name string, slot string, attr string) *Middleware {
	return &Middleware{
		slot: slot,
		name: name,
		attr: attr,
	}
}

type Middleware struct {
	slot     string
	name     string
	attr     string
	param    string
	config   *config.Config
	provider IProvider
}

func (m *Middleware) Provide(i interface{}) error {
	return m.provider.Provide(i)
}

// LoadConfig loads the configuration with the specified key and value into the Middleware's config.
//
// Parameters:
// - key: the key used to identify the configuration.
// - config: the configuration value to be loaded.
//
// Return type: None.
func (m *Middleware) LoadConfig(key string, config any) {
	_ = m.config.LoadWithKey(key, config)
}

func (m *Middleware) setConfig(conf *config.Config) {
	m.config = conf
}
func (m *Middleware) setProvider(provider IProvider) {
	m.provider = provider
}

func (m *Middleware) DoInitOnce() {

}

func (m *Middleware) doReg() {

	switch m.slot {
	case SlotMethod:
		attribute.RegAttributeType(m.attr, attribute.TypeMiddleware)
	case SlotController:
		attribute.RegAttributeType(m.attr, attribute.TypeMiddleware)
	case SlotGlobal:

	default:

	}
}

func (m *Middleware) Name() string {
	return m.name
}

func (m *Middleware) Attribute() AttributeName {
	return m.attr
}

func (m *Middleware) GetSlot() SlotType {
	return m.slot
}

type IMiddlewareMethod interface {
	IMiddleware
	Execute(ctx *MiddlewareContext) HandlerFunc
}

type IMiddlewareCtl interface {
	IMiddlewareMethod
	Router(ctx *MiddlewareContext) []*RouteItem
}

// RouteItem is a struct that holds information about a route
//
// a Middleware can return a []*RouteItem which will be registered to server
type RouteItem struct {
	Method           string         // HTTP METHOD
	Path             string         // route path
	IsHide           bool           // if set true, this route will not show in route table
	H                HandlerFunc    // handler for this route
	Middleware       IMiddlewareCtl // just refer to middleware itself
	OverrideBasePath bool           // override base path
}

// emptyRouteItem returns an empty []*RouteItem which won't register any route
func emptyRouteItem(m IMiddlewareCtl) []*RouteItem {
	return []*RouteItem{{
		Method:     "",
		Path:       "",
		IsHide:     false,
		H:          nil,
		Middleware: m,
	}}
}

func NewMiddlewareMethodForCtl(name string, attr string) *MiddlewareMethod {
	return &MiddlewareMethod{
		Middleware: NewMiddleware(name, SlotController, attr),
	}
}
func NewMiddlewareMethodForGlobal(name string) *MiddlewareMethod {
	return &MiddlewareMethod{
		Middleware: NewMiddleware(name, SlotGlobal, name),
	}
}

func NewMiddlewareCtlForGlobal(name string) *MiddlewareCtl {
	return &MiddlewareCtl{
		MiddlewareMethod: NewMiddlewareMethodForGlobal(name),
	}
}

func NewMiddlewareCtl(name string, attr string) *MiddlewareCtl {
	return &MiddlewareCtl{
		MiddlewareMethod: NewMiddlewareMethodForCtl(name, attr),
	}
}

type MiddlewareCtl struct {
	*MiddlewareMethod
}

func (m *MiddlewareCtl) Execute(ctx *MiddlewareContext) HandlerFunc {
	return ctx.Next
}

func (m *MiddlewareCtl) Router(ctx *MiddlewareContext) []*RouteItem {
	return emptyRouteItem(m)
}

func NewMiddlewareMethod(name string, attr string) *MiddlewareMethod {
	return &MiddlewareMethod{
		Middleware: NewMiddleware(name, SlotMethod, attr),
	}
}

type MiddlewareMethod struct {
	*Middleware
}

type IMiddlewareGlobal interface {
	IMiddlewareCtl
}

func NewMiddlewareGlobal(name string) *MiddlewareGlobal {
	return &MiddlewareGlobal{
		MiddlewareCtl: NewMiddlewareCtlForGlobal(name),
	}
}

type MiddlewareGlobal struct {
	*MiddlewareCtl
}

const (
	SlotGlobal     SlotType = "global"
	SlotController SlotType = "controller"
	SlotMethod     SlotType = "method"
)
