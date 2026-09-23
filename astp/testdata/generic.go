package testdata

// Stack 是一个泛型结构体，用于测试 Go 1.27 方法泛型的解析。
type Stack[T any] struct {
	items []T
}

// Push 是带自身类型参数 V 的泛型方法（Go 1.27 generic methods）。
// @POST
func (s *Stack[T]) Push[V any](v V) {}

// Pop 仅使用接收器(结构)的类型参数 T，方法自身没有类型参数。
// @GET
func (s *Stack[T]) Pop() T { return s.items[0] }

// Wrap 的方法自身类型参数约束是接口字面量联合约束（含 ~ 运算符）。
// 注意：Go 不允许类型形参作为约束（~T 与裸 T 均报 "cannot use a type parameter
// as constraint"），因此这里无法写成"约束引用结构类型参数 T"，改用具体类型的
// ~ 联合约束，同样覆盖 ~ 运算符与 InterfaceType 两条解析分支。
// @GET
func (s *Stack[T]) Wrap[U interface{ ~int | string }](u U) U { return u }

// Convert 有多个方法自身类型参数，且参数/返回值使用复合类型。
// @GET
func (s *Stack[T]) Convert[K comparable, V any](in []T) map[K]V { return nil }

// Ptr 的方法自身类型参数以指针形式出现在参数里。
// @GET
func (s *Stack[T]) Ptr[V any](v *V) {}

// Filter 是自由泛型函数，与方法共用同一套类型参数解析。
// @GET
func Filter[T any, R any](in []T, keep func(T) bool) []R { return nil }

// MapKeys 的自由函数类型参数约束是联合类型。
// @GET
func MapKeys[K int | string](in []K) map[K]bool { return nil }
