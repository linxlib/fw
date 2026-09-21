package astp

import (
	"path/filepath"
	"testing"
)

// findMethod 在解析结果里按名字找 Stack 的方法。
func findMethod(t *testing.T, stack *Type, name string) *Func {
	t.Helper()
	if stack == nil {
		t.Fatal("Stack type not found")
	}
	for _, m := range stack.Methods {
		if m.Name == name {
			return m
		}
	}
	t.Fatalf("method %s not found on Stack", name)
	return nil
}

func constraintNames(t *testing.T, gp *GenericParam) []string {
	t.Helper()
	if gp == nil {
		t.Fatalf("generic param is nil")
	}
	var out []string
	for _, c := range gp.Constraints {
		out = append(out, c.Name)
	}
	return out
}

func genericParamNamesOf(t *testing.T, fn *Func) []string {
	t.Helper()
	if fn == nil {
		t.Fatalf("func is nil")
	}
	var out []string
	for _, gp := range fn.Generic.Params {
		out = append(out, gp.Name)
	}
	return out
}

// TestGenericMethods 覆盖 Go 1.27 方法泛型（generic methods）的解析。
func TestGenericMethods(t *testing.T) {
	absDir, err := filepath.Abs(filepath.Join(".", "testdata"))
	if err != nil {
		t.Fatalf("get absolute path: %v", err)
	}

	project, err := NewParser().Parse(absDir)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	query := NewQuery(project)
	stack := query.FindType("Stack")
	if stack == nil {
		t.Fatal("expected to find Stack type")
	}
	if stack.Kind != KindStruct {
		t.Errorf("Stack kind = %v, want %v", stack.Kind, KindStruct)
	}

	// 结构体自身的类型参数仍然正确（回归）
	if stack.Generic == nil || len(stack.Generic.Params) != 1 || stack.Generic.Params[0].Name != "T" {
		t.Fatalf("Stack.Generic = %+v, want single param T", stack.Generic)
	}
	if names := constraintNames(t, stack.Generic.Params[0]); len(names) != 1 || names[0] != "any" {
		t.Errorf("Stack type param constraints = %v, want [any]", names)
	}

	t.Run("Push - 方法自身类型参数", func(t *testing.T) {
		push := findMethod(t, stack, "Push")

		if got := genericParamNamesOf(t, push); len(got) != 1 || got[0] != "V" {
			t.Fatalf("Push.Generic.Params = %v, want [V]", got)
		}
		// 约束现在会被保留（此前 parseFuncDecl 丢弃 Constraints）
		if names := constraintNames(t, push.Generic.Params[0]); len(names) != 1 || names[0] != "any" {
			t.Errorf("Push type param constraints = %v, want [any]", names)
		}

		// 参数 V 被标记为类型参数，而不是「具名结构体 V」
		if len(push.Params) != 1 {
			t.Fatalf("Push.Params len = %d, want 1", len(push.Params))
		}
		p := push.Params[0]
		if p.Name != "v" || p.Type.Name != "V" {
			t.Errorf("Push.Params[0] = %+v, want name=v type=V", p)
		}
		if p.Type.Kind != KindTypeParam {
			t.Errorf("Push.Params[0].Type.Kind = %v, want %v", p.Type.Kind, KindTypeParam)
		}
		if query.ResolveTypeRef(p.Type) != nil {
			t.Error("ResolveTypeRef on a type param should return nil")
		}

		// 接收器类型实参仍然正确，且被标记为类型参数（回归）
		if push.Recv == nil || push.Recv.Name != "Stack" {
			t.Fatalf("Push.Recv = %+v, want *Stack[T]", push.Recv)
		}
		if push.Recv.Generic == nil || len(push.Recv.Generic.Args) != 1 {
			t.Fatalf("Push.Recv.Generic = %+v, want one arg", push.Recv.Generic)
		}
		arg := push.Recv.Generic.Args[0]
		if arg.Name != "T" {
			t.Errorf("Push receiver arg = %v, want T", arg.Name)
		}
		if arg.Kind != KindTypeParam {
			t.Errorf("Push receiver arg kind = %v, want %v", arg.Kind, KindTypeParam)
		}
	})

	t.Run("Pop - 仅使用接收器类型参数", func(t *testing.T) {
		pop := findMethod(t, stack, "Pop")

		// 方法自身没有类型参数时 Generic 必须保持 nil
		if pop.Generic != nil {
			t.Errorf("Pop.Generic = %+v, want nil", pop.Generic)
		}
		if len(pop.Results) != 1 || pop.Results[0].Type.Name != "T" {
			t.Fatalf("Pop.Results = %+v, want single T", pop.Results)
		}
		if pop.Results[0].Type.Kind != KindTypeParam {
			t.Errorf("Pop.Results[0].Type.Kind = %v, want %v", pop.Results[0].Type.Kind, KindTypeParam)
		}
	})

	t.Run("Wrap - 约束引用结构类型参数", func(t *testing.T) {
		wrap := findMethod(t, stack, "Wrap")

		if got := genericParamNamesOf(t, wrap); len(got) != 1 || got[0] != "U" {
			t.Fatalf("Wrap.Generic.Params = %v, want [U]", got)
		}
		// ~T：保留被约束类型 T 的引用（~ 运算符本身不落库）
		if names := constraintNames(t, wrap.Generic.Params[0]); len(names) != 1 || names[0] != "T" {
			t.Errorf("Wrap type param constraints = %v, want [T]", names)
		}
		if wrap.Params[0].Type.Kind != KindTypeParam {
			t.Errorf("Wrap.Params[0].Type.Kind = %v, want %v", wrap.Params[0].Type.Kind, KindTypeParam)
		}
		if wrap.Results[0].Type.Kind != KindTypeParam {
			t.Errorf("Wrap.Results[0].Type.Kind = %v, want %v", wrap.Results[0].Type.Kind, KindTypeParam)
		}
	})

	t.Run("Convert - 多个类型参数与复合类型", func(t *testing.T) {
		convert := findMethod(t, stack, "Convert")

		if got := genericParamNamesOf(t, convert); len(got) != 2 || got[0] != "K" || got[1] != "V" {
			t.Fatalf("Convert.Generic.Params = %v, want [K V]", got)
		}
		if names := constraintNames(t, convert.Generic.Params[0]); len(names) != 1 || names[0] != "comparable" {
			t.Errorf("Convert K constraints = %v, want [comparable]", names)
		}
		if names := constraintNames(t, convert.Generic.Params[1]); len(names) != 1 || names[0] != "any" {
			t.Errorf("Convert V constraints = %v, want [any]", names)
		}

		// in []T：elem_type 是接收器类型参数
		in := convert.Params[0]
		if in.Type.Kind != KindSlice || in.Type.ElemType == nil || in.Type.ElemType.Name != "T" {
			t.Fatalf("Convert.Params[0].Type = %+v, want []T", in.Type)
		}
		if in.Type.ElemType.Kind != KindTypeParam {
			t.Errorf("Convert in elem kind = %v, want %v", in.Type.ElemType.Kind, KindTypeParam)
		}

		// map[K]V：key_type / elem_type 都是方法自身类型参数
		out := convert.Results[0]
		if out.Type.Kind != KindMap {
			t.Fatalf("Convert.Results[0].Type.Kind = %v, want %v", out.Type.Kind, KindMap)
		}
		if out.Type.KeyType == nil || out.Type.KeyType.Kind != KindTypeParam || out.Type.KeyType.Name != "K" {
			t.Errorf("Convert result key = %+v, want K typeparam", out.Type.KeyType)
		}
		if out.Type.ElemType == nil || out.Type.ElemType.Kind != KindTypeParam || out.Type.ElemType.Name != "V" {
			t.Errorf("Convert result elem = %+v, want V typeparam", out.Type.ElemType)
		}
	})

	t.Run("Ptr - 指针形式的方法类型参数", func(t *testing.T) {
		ptr := findMethod(t, stack, "Ptr")

		// 指针外层刻意不改写：指针形状对调用方更有价值
		if ptr.Params[0].Type.Kind != KindPointer || ptr.Params[0].Type.Name != "V" {
			t.Errorf("Ptr.Params[0].Type = %+v, want *V kept as pointer", ptr.Params[0].Type)
		}
	})

	t.Run("自由泛型函数回归", func(t *testing.T) {
		filter := query.FindFunc("Filter")
		if filter == nil {
			t.Fatal("expected to find Filter function")
		}
		if filter.Recv != nil {
			t.Errorf("Filter.Recv = %+v, want nil", filter.Recv)
		}
		if got := genericParamNamesOf(t, filter); len(got) != 2 || got[0] != "T" || got[1] != "R" {
			t.Fatalf("Filter.Generic.Params = %v, want [T R]", got)
		}
		if filter.Params[0].Type.ElemType.Kind != KindTypeParam {
			t.Errorf("Filter in elem kind = %v, want %v", filter.Params[0].Type.ElemType.Kind, KindTypeParam)
		}
		if filter.Results[0].Type.ElemType.Kind != KindTypeParam {
			t.Errorf("Filter out elem kind = %v, want %v", filter.Results[0].Type.ElemType.Kind, KindTypeParam)
		}

		// 联合约束 int | string 展开为两个约束引用
		mapKeys := query.FindFunc("MapKeys")
		if mapKeys == nil {
			t.Fatal("expected to find MapKeys function")
		}
		if names := constraintNames(t, mapKeys.Generic.Params[0]); len(names) != 2 || names[0] != "int" || names[1] != "string" {
			t.Errorf("MapKeys K constraints = %v, want [int string]", names)
		}
		if mapKeys.Params[0].Type.ElemType.Kind != KindTypeParam {
			t.Errorf("MapKeys in elem kind = %v, want %v", mapKeys.Params[0].Type.ElemType.Kind, KindTypeParam)
		}
	})

	t.Run("注解仍然可查", func(t *testing.T) {
		funcs := query.FindFuncsByAnnotation("GET")
		var found bool
		for _, f := range funcs {
			if f.Name == "Pop" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected Pop (a generic method) to be found by @GET annotation")
		}
	})
}

// TestGenericMethodsGeneratorLoader 验证泛型方法经过 JSON 往返后信息不丢失。
func TestGenericMethodsGeneratorLoader(t *testing.T) {
	absDir, err := filepath.Abs(filepath.Join(".", "testdata"))
	if err != nil {
		t.Fatalf("get absolute path: %v", err)
	}

	outputFile := filepath.Join(t.TempDir(), ".astp.json")
	if err := NewGenerator().Generate(absDir, outputFile); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	project, err := Load(outputFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	stack := NewQuery(project).FindType("Stack")
	if stack == nil {
		t.Fatal("expected to find Stack type after round-trip")
	}
	push := findMethod(t, stack, "Push")
	if push.Generic == nil || len(push.Generic.Params) != 1 || push.Generic.Params[0].Name != "V" {
		t.Fatalf("Push.Generic after round-trip = %+v, want single param V", push.Generic)
	}
	if push.Params[0].Type.Kind != KindTypeParam {
		t.Errorf("Push.Params[0].Type.Kind after round-trip = %v, want %v", push.Params[0].Type.Kind, KindTypeParam)
	}
}
