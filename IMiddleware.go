package fw

import "github.com/linxlib/fw/attribute"

/// TODO:
/// 中间件需要可以从注释的位置传参进来
/// 例如： @Logger color=false&params=body1,path1
/// @BasicAuth user=admin&pass=admin
/// 参数可以采用query的形式传入（暂时），或者其他方式

/// 中间件需要由系统注入一些常用的服务，例如数据库之类的，这个不方便在参数处传入
/// 当然如果inject可以根据名称去注入的话

type AttributeName = string
type SlotType = string

// IMiddleware
// interface of middleware
type IMiddleware interface {
	// Name returns middleware's name
	Name() string
	// Attribute returns middleware's Attribute just like Websocket so that you can use it like // @Websocket
	Attribute() AttributeName
	Slot(string) bool
	GetSlot() SlotType
	SetParam(string)
	GetParam() string
	doReg()
}
type IMiddlewareMethod interface {
	IMiddleware
	CloneAsMethod() IMiddlewareMethod
	HandlerMethod(h HandlerFunc) HandlerFunc
}
type IMiddlewareCtl interface {
	IMiddlewareMethod
	CloneAsCtl() IMiddlewareCtl
	HandlerController(string) *RouteItem
}

type IMiddlewareInject interface {
	IMiddlewareMethod
}

type IMiddlewareGlobal interface {
	IMiddlewareCtl
}

type RouteItem struct {
	Method     string //HTTP METHOD
	Path       string // route path
	IsHide     bool   // if set true, this route will not be print
	H          HandlerFunc
	Middleware IMiddlewareMethod
}

var _ IMiddleware = (*Middleware)(nil)

func NewMiddleware(name string, slot string, attr string) Middleware {
	return Middleware{
		slot: slot,
		name: name,
		attr: attr,
	}
}

func NewMiddlewareMethod(name string, attr string) MiddlewareMethod {
	return MiddlewareMethod{
		Middleware: NewMiddleware(name, SlotMethod, attr),
	}
}
func NewMiddlewareCtl(name string, attr string) MiddlewareCtl {
	return MiddlewareCtl{
		Middleware: NewMiddleware(name, SlotController, attr),
	}
}

func NewMiddlewareGlobal(name string) MiddlewareGlobal {
	return MiddlewareGlobal{
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
		AddMethodAttributeType(m.attr, attribute.TypeMiddleware)
	case SlotController:
		AddCtlAttributeType(m.attr, attribute.TypeMiddleware)
	case SlotGlobal:

	default:

	}
}

func (m *Middleware) GetParam() string {
	return m.param
}

func (m *Middleware) SetName(s string) {
	m.name = s
}

func (m *Middleware) SetAttribute(name AttributeName) {
	m.attr = name
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

func (m *Middleware) Slot(s string) bool {
	return m.slot == s
}

func (m *Middleware) GetSlot() SlotType {
	return m.slot
}

func (m *Middleware) SetParam(s string) {
	m.param = s
}

func (m *Middleware) Invoke(h HandlerFunc) HandlerFunc {
	return nil
}

type MiddlewareCtl struct {
	Middleware
}

type MiddlewareMethod struct {
	Middleware
}

type MiddlewareGlobal struct {
	Middleware
}

const (
	SlotGlobal     SlotType = "global"
	SlotController SlotType = "controller"
	SlotMethod     SlotType = "method"
)
