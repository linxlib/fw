package testdata

// Base 是被嵌入的泛型基础结构体, 用于测试方法提升(method promotion).
type Base[T any] struct {
	items []T
}

// Name 返回基础结构体自身的方法.
// @GET /base/name
func (b *Base[T]) Name() string { return "base" }

// Count 返回元素个数, 引用结构体自身的类型参数 T.
// @GET /base/count
func (b *Base[T]) Count() int { return len(b.items) }

// Middle 嵌入 Base, 形成两层提升链.
type Middle[T any] struct {
	Base[T]
	label string
}

// Label 是 Middle 自身声明的方法.
// @GET /middle/label
func (m *Middle[T]) Label() string { return m.label }

// Outer 嵌入 Middle, 应同时获得 Middle 自身的方法与 Base 提升上来的方法.
type Outer[T any] struct {
	Middle[T]
}

// Name 遮蔽 Base.Name: 外层显式声明的方法优先, 距离更近的 Middle 也优先.
// @GET /outer/name
func (o *Outer[T]) Name() string { return "outer" }

// Own 是 Outer 自身声明的方法.
// @GET /outer/own
func (o *Outer[T]) Own() string { return "outer" }

// Plain 是非泛型结构体, 嵌入泛型 Base 后同样获得提升方法.
type Plain struct {
	Base[string]
}

// Describe 是 Plain 自身声明的方法.
// @GET /plain/describe
func (p *Plain) Describe() string { return "plain" }
