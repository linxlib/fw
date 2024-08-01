package fw

import "github.com/linxlib/fw/attribute"

type AttributeName = string
type SlotType = string

// IMiddleware
// interface of middleware
type IMiddleware interface {
	// Name returns middleware's name
	Name() string
	// Attribute returns middleware's Attribute just like Websocket so that you can use it like // @Websocket
	Attribute() AttributeName
	// GetSlot returns slot type
	GetSlot() SlotType
	// SetParam pass params (strings with query format) to middleware
	SetParam(p string)
	// GetParam return params string
	GetParam() string
	// doReg inner called by fw
	doReg()
}
type IMiddlewareMethod interface {
	IMiddleware
	// CloneAsMethod returns a copy from Middleware Container
	CloneAsMethod() IMiddlewareMethod
	// HandlerMethod will be called when wrap a method
	HandlerMethod(next HandlerFunc) HandlerFunc
}
type IMiddlewareCtl interface {
	IMiddlewareMethod
	// CloneAsCtl returns a copy from Middleware Container
	CloneAsCtl() IMiddlewareCtl
	// HandlerController will be called when handling controller
	// returns many RouteItem(field `Path` is not empty) if you want to register a route
	HandlerController(base string) []*RouteItem
}

type IMiddlewareGlobal interface {
	IMiddlewareCtl
}

type RouteItem struct {
	Method     string            // HTTP METHOD
	Path       string            // route path
	IsHide     bool              // if set true, this route will not show in route table
	H          HandlerFunc       // handler for this route
	Middleware IMiddlewareMethod // just refer to middleware itself
}

// EmptyRouteItem returns an empty RouteItem which won't register route
func EmptyRouteItem(m IMiddlewareMethod) []*RouteItem {
	return []*RouteItem{{
		Method:     "",
		Path:       "",
		IsHide:     false,
		H:          nil,
		Middleware: m,
	}}
}

var _ IMiddleware = (*Middleware)(nil)

func NewMiddleware(name string, slot string, attr string) *Middleware {
	return &Middleware{
		slot: slot,
		name: name,
		attr: attr,
	}
}

func NewMiddlewareMethod(name string, attr string) *MiddlewareMethod {
	return &MiddlewareMethod{
		Middleware: NewMiddleware(name, SlotMethod, attr),
	}
}
func NewMiddlewareCtl(name string, attr string) *MiddlewareCtl {
	return &MiddlewareCtl{
		Middleware: NewMiddleware(name, SlotController, attr),
	}
}

func NewMiddlewareGlobal(name string) *MiddlewareGlobal {
	return &MiddlewareGlobal{
		Middleware: NewMiddleware(name, SlotGlobal, ""),
	}
}

type Middleware struct {
	slot  string
	name  string
	attr  string
	param string
}

func (m *Middleware) doReg() {
	switch m.slot {
	case SlotMethod:
		attribute.AddMethodAttributeType(m.attr, attribute.TypeMiddleware)
	case SlotController:
		attribute.AddStructAttributeType(m.attr, attribute.TypeMiddleware)
	case SlotGlobal:

	default:

	}
}

func (m *Middleware) GetParam() string {
	return m.param
}

func (m *Middleware) SetName(name string) {
	m.name = name
}

func (m *Middleware) SetAttribute(attr AttributeName) {
	m.attr = attr
}

func (m *Middleware) SetSlot(slotType SlotType) {
	m.slot = slotType
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

func (m *Middleware) SetParam(p string) {
	m.param = p
}

type MiddlewareCtl struct {
	*Middleware
}

type MiddlewareMethod struct {
	*Middleware
}

type MiddlewareGlobal struct {
	*Middleware
}

const (
	SlotGlobal     SlotType = "global"
	SlotController SlotType = "controller"
	SlotMethod     SlotType = "method"
)
