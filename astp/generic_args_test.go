package astp

import (
	"path/filepath"
	"testing"
)

// findGenericArgsMethod 按名字找 owner 上的方法.
func findGenericArgsMethod(t *testing.T, owner *Type, name string) *Func {
	t.Helper()
	for _, m := range owner.Methods {
		if m != nil && m.Name == name {
			return m
		}
	}
	t.Fatalf("method %s not found on %s", name, owner.Name)
	return nil
}

// TestRecvTypeArgs 覆盖泛型实参推断: 从业务类型的嵌入字段还原方法签名里的类型占位.
func TestRecvTypeArgs(t *testing.T) {
	absDir, err := filepath.Abs(filepath.Join(".", "testdata"))
	if err != nil {
		t.Fatalf("get absolute path: %v", err)
	}
	project, err := NewParser().Parse(absDir)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	q := NewQuery(project)

	userCtrl := q.FindType("UserCtrl")
	if userCtrl == nil {
		t.Fatal("expected to find UserCtrl")
	}
	nestedCtrl := q.FindType("NestedCtrl")
	if nestedCtrl == nil {
		t.Fatal("expected to find NestedCtrl")
	}
	recursiveCtrl := q.FindType("RecursiveCtrl")
	if recursiveCtrl == nil {
		t.Fatal("expected to find RecursiveCtrl")
	}
	baseCtrl := q.FindType("BaseCtrl")
	if baseCtrl == nil {
		t.Fatal("expected to find BaseCtrl")
	}

	t.Run("裸类型参数", func(t *testing.T) {
		bind := RecvTypeArgs(q, userCtrl, findGenericArgsMethod(t, userCtrl, "One"))
		if bind == nil || len(bind) != 1 {
			t.Fatalf("bind = %v, want single entry", bind)
		}
		if bind["T"] == nil || bind["T"].Name != "Member" {
			t.Fatalf("bind[T] = %v, want Member", bind["T"])
		}

		got := InstantiateTypeRef(findGenericArgsMethod(t, userCtrl, "One").Results[0].Type, bind)
		if got == nil || got.Name != "Member" {
			t.Fatalf("instantiated One() result = %v, want Member", got)
		}
	})

	t.Run("指针类型参数保留 pointer 形状", func(t *testing.T) {
		m := findGenericArgsMethod(t, userCtrl, "Create")
		bind := RecvTypeArgs(q, userCtrl, m)
		got := InstantiateTypeRef(m.Params[0].Type, bind)
		if got == nil || got.Name != "Member" || got.Kind != KindPointer {
			t.Fatalf("instantiated Create param = %v, want *Member (pointer)", got)
		}
	})

	t.Run("切片元素类型", func(t *testing.T) {
		m := findGenericArgsMethod(t, userCtrl, "Slice")
		bind := RecvTypeArgs(q, userCtrl, m)
		got := InstantiateTypeRef(m.Results[0].Type, bind)
		if got == nil || got.Kind != KindSlice {
			t.Fatalf("instantiated Slice() = %v, want slice", got)
		}
		if got.ElemType == nil || got.ElemType.Name != "Member" || got.ElemType.Kind != KindPointer {
			t.Fatalf("slice elem = %v, want *Member", got.ElemType)
		}
	})

	t.Run("泛型实例化嵌套", func(t *testing.T) {
		m := findGenericArgsMethod(t, userCtrl, "Page")
		bind := RecvTypeArgs(q, userCtrl, m)
		got := InstantiateTypeRef(m.Results[0].Type, bind)
		if got == nil || got.Name != "PageSize" || got.Kind != KindStruct {
			t.Fatalf("instantiated Page() = %v, want PageSize", got)
		}
		if got.Generic == nil || len(got.Generic.Args) != 1 {
			t.Fatalf("PageSize args = %v, want one arg", got.Generic)
		}
		arg := got.Generic.Args[0]
		// *T 替换后必须仍是 pointer, 且名字是 User.
		if arg.Name != "Member" || arg.Kind != KindPointer {
			t.Fatalf("PageSize arg = %v, want *Member", arg)
		}
	})

	t.Run("同名不同作用域", func(t *testing.T) {
		// Page() 返回 PageSize[*T]: 替换后 PageSize 的实参是 *User.
		// 接着展开 PageSize 自身字段时, 必须用 PageSize 的形参 T 重新绑定,
		// 否则会把 BaseCtrl 的 T 误用到 PageSize 作用域.
		pageSize := q.FindType("PageSize")
		if pageSize == nil {
			t.Fatal("expected to find PageSize")
		}
		m := findGenericArgsMethod(t, userCtrl, "Page")
		bind := RecvTypeArgs(q, userCtrl, m)
		outer := InstantiateTypeRef(m.Results[0].Type, bind)
		if len(outer.Generic.Args) != 1 {
			t.Fatalf("PageSize args = %v, want 1", outer.Generic.Args)
		}

		inner := RebindTypeArgs(pageSize, outer.Generic.Args)
		if inner == nil || len(inner) != 1 || inner["T"] == nil {
			t.Fatalf("rebound inner binding = %v, want T bound", inner)
		}
		fields := InstantiateFields(pageSize.Fields, inner)
		items := fields[len(fields)-1]
		if items.Name != "Items" {
			t.Fatalf("last field = %s, want Items", items.Name)
		}
		if items.Type.Kind != KindSlice {
			t.Fatalf("Items kind = %v, want slice", items.Type.Kind)
		}
		if items.Type.ElemType == nil || items.Type.ElemType.Name != "Member" {
			t.Fatalf("Items elem = %v, want Member", items.Type.ElemType)
		}
		if items.Type.ElemType.Kind != KindPointer {
			t.Fatalf("Items elem kind = %v, want pointer", items.Type.ElemType.Kind)
		}
	})

	t.Run("多级嵌套 PageSize[Account[int]]", func(t *testing.T) {
		m := findGenericArgsMethod(t, userCtrl, "PlainPage")
		bind := RecvTypeArgs(q, userCtrl, m)
		got := InstantiateTypeRef(m.Results[0].Type, bind)
		if got == nil || got.Name != "PageSize" {
			t.Fatalf("instantiated PlainPage() = %v, want PageSize", got)
		}
		// 实参本身还带泛型实参: Account[int]
		arg := got.Generic.Args[0]
		if arg.Name != "Account" {
			t.Fatalf("PageSize arg = %v, want Account", arg)
		}
		if arg.Generic == nil || len(arg.Generic.Args) != 1 {
			t.Fatalf("Account args = %v, want one arg", arg.Generic)
		}
		if arg.Generic.Args[0].Name != "int" || arg.Generic.Args[0].Kind != KindBasic {
			t.Fatalf("Account arg = %v, want int", arg.Generic.Args[0])
		}

		// 展开 Account 自身字段时, Code 的 T 应解析为 int
		account := q.FindType("Account")
		if account == nil {
			t.Fatal("expected to find Account")
		}
		inner := RebindTypeArgs(account, arg.Generic.Args)
		fields := InstantiateFields(account.Fields, inner)
		if fields[0].Name != "Code" {
			t.Fatalf("first field = %s, want Code", fields[0].Name)
		}
		if fields[0].Type.Name != "int" || fields[0].Type.Kind != KindBasic {
			t.Fatalf("Code type = %v, want int", fields[0].Type)
		}
	})

	t.Run("map 与 slice 混合嵌套", func(t *testing.T) {
		m := findGenericArgsMethod(t, userCtrl, "Mixed")
		bind := RecvTypeArgs(q, userCtrl, m)
		got := InstantiateTypeRef(m.Results[0].Type, bind)
		if got == nil || got.Kind != KindMap {
			t.Fatalf("instantiated Mixed() = %v, want map", got)
		}
		inner := got.ElemType
		if inner == nil || inner.Kind != KindSlice {
			t.Fatalf("map elem = %v, want slice", inner)
		}
		pageSize := inner.ElemType
		if pageSize == nil || pageSize.Name != "PageSize" {
			t.Fatalf("slice elem = %v, want PageSize", pageSize)
		}
		if pageSize.Generic.Args[0].Name != "Member" {
			t.Fatalf("PageSize arg = %v, want Member", pageSize.Generic.Args[0])
		}
	})

	t.Run("自引用泛型不爆栈", func(t *testing.T) {
		m := findGenericArgsMethod(t, recursiveCtrl, "Rec")
		bind := RecvTypeArgs(q, recursiveCtrl, m)
		if bind == nil {
			t.Fatal("expected a binding for RecursiveCtrl")
		}
		// 必须能在有限时间内完成, 不 panic / 不 stack overflow.
		got := InstantiateTypeRef(m.Results[0].Type, bind)
		if got == nil || got.Name != "Node" {
			t.Fatalf("instantiated Rec() = %v, want Node", got)
		}
		if got.Kind != KindPointer {
			t.Fatalf("Rec() kind = %v, want pointer", got.Kind)
		}
	})

	t.Run("自身有类型参数时无绑定", func(t *testing.T) {
		m := findGenericArgsMethod(t, userCtrl, "SelfGeneric")
		if bind := RecvTypeArgs(q, userCtrl, m); bind != nil {
			t.Fatalf("bind = %v, want nil (method declares its own type params)", bind)
		}
	})

	t.Run("未实例化的泛型基类无绑定", func(t *testing.T) {
		m := findGenericArgsMethod(t, baseCtrl, "One")
		if bind := RecvTypeArgs(q, baseCtrl, m); bind != nil {
			t.Fatalf("bind = %v, want nil (base class registered without instantiation)", bind)
		}
	})

	t.Run("嵌套实例化 BaseCtrl[Account[int]]", func(t *testing.T) {
		m := findGenericArgsMethod(t, nestedCtrl, "One")
		bind := RecvTypeArgs(q, nestedCtrl, m)
		if bind == nil || bind["T"] == nil {
			t.Fatalf("bind = %v, want T bound", bind)
		}
		got := InstantiateTypeRef(m.Results[0].Type, bind)
		if got.Name != "Account" {
			t.Fatalf("instantiated One() = %v, want Account", got)
		}
		if got.Generic == nil || len(got.Generic.Args) != 1 || got.Generic.Args[0].Name != "int" {
			t.Fatalf("Account args = %v, want [int]", got.Generic)
		}
	})
}

// TestInstantiateTypeRefImmutability 确认实例化不会污染原始元数据.
func TestInstantiateTypeRefImmutability(t *testing.T) {
	original := &TypeRef{
		Name: "PageSize",
		Kind: KindStruct,
		Generic: &GenericArg{Args: []*TypeRef{
			{Name: "T", Kind: KindPointer},
		}},
	}
	bind := TypeArgBinding{"T": {Name: "Member", Kind: KindStruct}}

	got := InstantiateTypeRef(original, bind)
	if got == original {
		t.Fatal("expected a copy, got the same pointer")
	}
	if got.Generic.Args[0].Name != "Member" {
		t.Errorf("instantiated arg = %v, want Member", got.Generic.Args[0].Name)
	}
	// 原始元数据必须保持占位不变.
	if original.Generic.Args[0].Name != "T" {
		t.Errorf("original arg mutated to %v, want T", original.Generic.Args[0].Name)
	}
}

// TestInstantiateTypeRefNoBinding 确认空绑定是零成本直通.
func TestInstantiateTypeRefNoBinding(t *testing.T) {
	ref := &TypeRef{Name: "Member", Kind: KindStruct}
	if got := InstantiateTypeRef(ref, nil); got != ref {
		t.Error("nil binding should return the same pointer")
	}
	if got := InstantiateTypeRef(ref, TypeArgBinding{}); got != ref {
		t.Error("empty binding should return the same pointer")
	}
}
