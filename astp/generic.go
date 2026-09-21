package astp

// 泛型实参推断: 把方法签名里的「类型形参占位」还原成 owner 类型上的「实际类型实参」。
//
// 背景: 泛型基础结构体的能力通过方法提升被业务类型继承, 但方法签名里的 T 始终是占位。
// 例如
//
//	type BaseController[T any] struct{ /* ... */ }
//	func (c *BaseController[T]) List() PageSize[*T] { /* ... */ }
//
//	type UserController struct {
//	    BaseController[models.User]     // 这里才知道 T = models.User
//	}
//
// 因此必须在「拥有该方法的业务类型」这一层, 从嵌入字段上取出实参链, 再按名字替换签名里的
// 占位。替换必须按名字而不是按 Kind: *T 落库为 {name:"T", kind:"pointer"},
// 裸 T 落库为 {name:"T", kind:"typeparam"}, 两者都要替换。

// maxTypeArgDepth 限制实参替换的递归深度, 防止 type Node[T any] struct{ Next *Node[T] }
// 这类自引用泛型无限展开.
const maxTypeArgDepth = 8

// TypeArgBinding 把类型形参名映射到实际类型实参.
type TypeArgBinding map[string]*TypeRef

// RecvTypeArgs 推断 fn 在 owner(拥有该方法的类型, 如 UserController)上下文中的类型实参.
//
// 返回 nil 表示「没有可用绑定」, 调用方应保持静默降级(不报错, 也不 panic).
// 以下情况会返回 nil:
//
//   - fn 自身声明了类型参数(Go 1.27 泛型方法): 具体类型只在调用点可知, 静态阶段无从推断;
//   - fn 没有接收者(自由函数): 无从查找实例化来源;
//   - owner 与 fn 的接收器基类型相同: 说明直接注册了泛型基类而未实例化;
//   - 在 owner 的嵌入字段里找不到该接收器基类型的实例化;
//   - 形参数量与实参数量不一致.
func RecvTypeArgs(q *Query, owner *Type, fn *Func) TypeArgBinding {
	if q == nil || owner == nil || fn == nil {
		return nil
	}
	// 方法自身有类型参数 → 具体类型在调用点才知道.
	if fn.Generic != nil && len(fn.Generic.Params) > 0 {
		return nil
	}
	if fn.Recv == nil || fn.Recv.Name == "" {
		return nil
	}

	baseName := fn.Recv.Name
	// 直接注册泛型基类(owner 就是接收器基类型) → 未实例化.
	if owner.Name == baseName {
		return nil
	}

	base := q.FindType(baseName)
	if base == nil || base.Generic == nil || len(base.Generic.Params) == 0 {
		// 基类型不是泛型, 或本模块内解析不到: 没有可推断的形参.
		return nil
	}

	args := findEmbeddedTypeArgs(owner, baseName)
	if len(args) == 0 {
		return nil
	}
	if len(args) != len(base.Generic.Params) {
		return nil
	}

	bind := make(TypeArgBinding, len(args))
	for i, param := range base.Generic.Params {
		if param == nil || param.Name == "" {
			continue
		}
		bind[param.Name] = args[i]
	}
	return bind
}

// findEmbeddedTypeArgs 在 owner 的嵌入字段里找名为 baseName 的类型, 返回它的实参列表.
func findEmbeddedTypeArgs(owner *Type, baseName string) []*TypeRef {
	for _, f := range owner.Fields {
		if f == nil || !f.Embedded || f.Type == nil {
			continue
		}
		if f.Type.Name != baseName {
			continue
		}
		if f.Type.Generic == nil {
			return nil
		}
		return f.Type.Generic.Args
	}
	return nil
}

// InstantiateTypeRef 按名字递归替换 ref 中的类型形参, 保留原 Kind(*T 替换后仍是 pointer).
//
// 覆盖 Generic.Args / KeyType / ElemType. 带深度上限与循环保护, 递归泛型不会爆栈.
// bind 为 nil 或空时返回 ref 本身(不复制), 保持零成本.
func InstantiateTypeRef(ref *TypeRef, bind TypeArgBinding) *TypeRef {
	if ref == nil || len(bind) == 0 {
		return ref
	}
	return instantiateRef(ref, bind, 0, nil)
}

func instantiateRef(ref *TypeRef, bind TypeArgBinding, depth int, visiting map[*TypeRef]bool) *TypeRef {
	if ref == nil {
		return nil
	}
	if depth > maxTypeArgDepth {
		return ref
	}
	if visiting == nil {
		visiting = make(map[*TypeRef]bool)
	}
	if visiting[ref] {
		return ref
	}
	visiting[ref] = true
	defer delete(visiting, ref)

	// 深拷贝, 不改动调用方传入的原始元数据(它可能被多个路由共享).
	out := &TypeRef{
		Name:     ref.Name,
		PkgPath:  ref.PkgPath,
		Kind:     ref.Kind,
		KeyType:  instantiateRef(ref.KeyType, bind, depth+1, visiting),
		ElemType: instantiateRef(ref.ElemType, bind, depth+1, visiting),
	}

	if ref.Generic != nil {
		out.Generic = &GenericArg{Args: make([]*TypeRef, 0, len(ref.Generic.Args))}
		for _, arg := range ref.Generic.Args {
			out.Generic.Args = append(out.Generic.Args, instantiateRef(arg, bind, depth+1, visiting))
		}
	}

	// 按名字替换: 命中绑定即视为「这里是该形参的占位」.
	//
	// 是否采用实参的 Kind, 取决于本层是不是一个「裸占位」:
	//
	//   - 裸占位(ref.Kind == KindTypeParam, 或字段侧那种长得像 struct 的裸名字):
	//     完全采用实参的名字/包路径/形状. 占位本身没有类型语义, T 换成 int 后
	//     kind 就该是 basic.
	//   - 非裸(外层是 pointer/slice/map 等, 如 *T 落库为 {name:"T", kind:"pointer"}):
	//     保留本层 Kind 让外层形状延续, 只替换名字并带入实参的泛型实参.
	//
	// 注意字段侧的类型参数引用落库为 kind:"struct"(astp 当前不对字段做类型参数标记),
	// 因此这里不能只靠 Kind 判断, 绑定集合本身就是权威依据.
	if repl, ok := bind[ref.Name]; ok && ref.PkgPath == "" {
		if repl != nil {
			bare := ref.Kind == KindTypeParam ||
				(ref.Kind == KindStruct && ref.Generic == nil && ref.KeyType == nil && ref.ElemType == nil)
			out.Name = repl.Name
			out.PkgPath = repl.PkgPath
			if bare {
				out.Kind = repl.Kind
			}
			// 实参自身可能还带泛型实参(如 Account[int]), 一并带入.
			if repl.Generic != nil {
				out.Generic = instantiateRef(repl, bind, depth+1, visiting).Generic
			}
		}
	}

	return out
}

// InstantiateFields 批量替换字段类型, 保留 Name / Tag / Doc / Embedded 等元数据.
func InstantiateFields(fields []*Field, bind TypeArgBinding) []*Field {
	if len(fields) == 0 || len(bind) == 0 {
		return fields
	}
	out := make([]*Field, 0, len(fields))
	for _, f := range fields {
		if f == nil {
			out = append(out, nil)
			continue
		}
		nf := &Field{
			Name:     f.Name,
			Type:     InstantiateTypeRef(f.Type, bind),
			Tag:      f.Tag,
			Doc:      f.Doc,
			Embedded: f.Embedded,
		}
		out = append(out, nf)
	}
	return out
}

// RebindTypeArgs 用被实例化类型自身的形参列表与传入实参重新建立绑定.
//
// 这是处理「同名不同作用域」的关键: Base.T 与 PageSize.T 是两个独立的类型参数,
// 进入 PageSize 时必须用 PageSize.Generic.Params 重新配对, 否则会把 Base 的绑定
// 误用到 PageSize 的作用域里.
//
// 形参为空、实参为空或数量不一致时返回 nil(表示这一层无可推断绑定).
func RebindTypeArgs(t *Type, args []*TypeRef) TypeArgBinding {
	if t == nil || t.Generic == nil || len(t.Generic.Params) == 0 {
		return nil
	}
	if len(args) == 0 || len(args) != len(t.Generic.Params) {
		return nil
	}
	bind := make(TypeArgBinding, len(args))
	for i, param := range t.Generic.Params {
		if param == nil || param.Name == "" {
			continue
		}
		bind[param.Name] = args[i]
	}
	return bind
}
