package testdata

// Member 是被实例化的业务实体(刻意不与 testdata/sample.go 的 User 重名).
type Member struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Account 是带自身泛型实参的 model, 用于多级嵌套.
type Account[T int | string] struct {
	Code T      `json:"code"`
	Desc string `json:"desc"`
}

// PageSize 是泛型分页 model. 注意它的类型参数也叫 T, 与 BaseCtrl 的 T 同名但不同作用域.
type PageSize[T any] struct {
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
	Items []T   `json:"items"`
}

// BaseCtrl 是泛型基础控制器.
type BaseCtrl[T any] struct {
	items []T
}

// One 返回裸类型参数.
// @GET /one
func (c *BaseCtrl[T]) One() T { var z T; return z }

// Create 的请求体是指针类型参数.
// @POST /
func (c *BaseCtrl[T]) Create(entity *T) error { return nil }

// Slice 返回切片.
// @GET /slice
func (c *BaseCtrl[T]) Slice() []*T { return nil }

// Page 返回泛型实例化 PageSize[*T] —— PageSize 的 T 与 BaseCtrl 的 T 同名但不同作用域.
// @GET /page
func (c *BaseCtrl[T]) Page() PageSize[*T] { return PageSize[*T]{} }

// PlainPage 返回直接实例化的多级嵌套 PageSize[Account[int]].
// @GET /plain_page
func (c *BaseCtrl[T]) PlainPage() PageSize[Account[int]] { return PageSize[Account[int]]{} }

// Mixed 返回 map + slice + 泛型嵌套.
// @GET /mixed
func (c *BaseCtrl[T]) Mixed() map[string][]PageSize[T] { return nil }

// SelfGeneric 自身声明类型参数(Go 1.27 泛型方法), 无法静态推断.
// @GET /self
func (c *BaseCtrl[T]) SelfGeneric[V any](v V) V { return v }

// UserCtrl 内嵌实例化后的 BaseCtrl, 提升方法在这里才知道 T = Member.
// @Controller
// @Route /user
type UserCtrl struct {
	BaseCtrl[Member]
}

// NestedCtrl 内嵌多级实例化 BaseCtrl[Account[int]].
// @Controller
// @Route /nested
type NestedCtrl struct {
	BaseCtrl[Account[int]]
}

// Node 是自引用泛型, 用于验证不会无限递归.
type Node[T any] struct {
	Val  T        `json:"val"`
	Next *Node[T] `json:"next"`
}

// RecursiveCtrl 内嵌 Node, 验证递归泛型不爆栈.
// @Controller
// @Route /rec
type RecursiveCtrl struct {
	BaseCtrl[Node[int]]
}

// Rec 返回自引用泛型.
// @GET /rec
func (c *BaseCtrl[T]) Rec() *Node[T] { return nil }
