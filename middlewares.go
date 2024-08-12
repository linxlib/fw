package fw

import "strings"

// MiddlewareContainer stores middlewares
// global middlewares will be stored with its Name as key
type MiddlewareContainer struct {
	ms map[SlotType]map[string]IMiddleware
}

func NewMiddlewareContainer() *MiddlewareContainer {
	m := &MiddlewareContainer{
		ms: make(map[SlotType]map[string]IMiddleware),
	}
	m.ms[SlotGlobal] = make(map[string]IMiddleware)
	m.ms[SlotController] = make(map[AttributeName]IMiddleware)
	m.ms[SlotMethod] = make(map[AttributeName]IMiddleware)
	return m
}

// Reg will store middleware to specified map according to its slot
func (m *MiddlewareContainer) Reg(middleware IMiddleware) {
	middleware.doReg()
	st := middleware.GetSlot()
	switch st {
	case SlotGlobal:
		m.ms[st][middleware.Name()] = middleware
	case SlotController:
		m.ms[st][strings.ToUpper(middleware.Attribute())] = middleware
	case SlotMethod:
		m.ms[st][strings.ToUpper(middleware.Attribute())] = middleware
	}

}

// VisitAll visit all middlewares with the specified slot
// if f returns true, the loop will terminate
func (m *MiddlewareContainer) VisitAll(slot string, f func(middleware IMiddleware) bool) bool {
	for _, middleware := range m.ms[slot] {
		if f(middleware) == true {
			return true
		}
	}
	return false
}

// GetByAttribute returns middleware with specified slot and attr.
func (m *MiddlewareContainer) GetByAttribute(slot string, attribute string) (IMiddleware, bool) {
	if mid, ok := m.ms[slot][attribute]; ok {
		return mid, ok
	} else {
		return nil, false
	}
}

// GetGlobal iterate global middlewares
// stop at `f` returns true
func (m *MiddlewareContainer) GetGlobal(f func(middleware IMiddlewareGlobal) bool) bool {
	for _, middleware := range m.ms[SlotGlobal] {
		if f(middleware.(IMiddlewareGlobal)) == true {
			return true
		}
	}
	return false
}

// GetByAttributeCtl find controller middleware(IMiddlewareCtl) with attribute name
func (m *MiddlewareContainer) GetByAttributeCtl(attribute string) (IMiddlewareCtl, bool) {
	if mid, ok := m.ms[SlotController][attribute]; ok {
		return mid.(IMiddlewareCtl), ok
	} else {
		return nil, false
	}
}

// GetByAttributeMethod find method middleware(IMiddlewareMethod) with attribute name
func (m *MiddlewareContainer) GetByAttributeMethod(attribute string) (IMiddlewareMethod, bool) {
	if mid, ok := m.ms[SlotMethod][attribute]; ok {
		return mid.(IMiddlewareMethod), ok
	} else {
		return nil, false
	}
}
