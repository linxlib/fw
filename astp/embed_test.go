package astp

import (
	"path/filepath"
	"sort"
	"testing"
)

// methodNames 返回类型方法名的有序列表.
func methodNames(methods []*Func) []string {
	out := make([]string, 0, len(methods))
	for _, m := range methods {
		if m != nil {
			out = append(out, m.Name)
		}
	}
	sort.Strings(out)
	return out
}

func findEmbeddedMethod(t *testing.T, typ *Type, name string) *Func {
	t.Helper()
	for _, m := range typ.Methods {
		if m != nil && m.Name == name {
			return m
		}
	}
	t.Fatalf("method %s not promoted onto %s (have %v)", name, typ.Name, methodNames(typ.Methods))
	return nil
}

// TestEmbeddedMethodPromotion 覆盖嵌入字段的方法提升(含泛型基础结构体).
func TestEmbeddedMethodPromotion(t *testing.T) {
	absDir, err := filepath.Abs(filepath.Join(".", "testdata"))
	if err != nil {
		t.Fatalf("get absolute path: %v", err)
	}

	project, err := NewParser().Parse(absDir)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	q := NewQuery(project)

	t.Run("两层提升链", func(t *testing.T) {
		outer := q.FindType("Outer")
		if outer == nil {
			t.Fatal("expected to find Outer type")
		}
		got := methodNames(outer.Methods)
		want := []string{"Count", "Label", "Name", "Own"}
		if len(got) != len(want) {
			t.Fatalf("Outer methods = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Outer methods = %v, want %v", got, want)
			}
		}
	})

	t.Run("外层显式方法遮蔽提升方法", func(t *testing.T) {
		outer := q.FindType("Outer")
		if outer == nil {
			t.Fatal("expected to find Outer type")
		}
		name := findEmbeddedMethod(t, outer, "Name")
		// 提升自 Base 的方法 Recv 是 *Base; Outer 自身声明的 Name 也共享同一个 *Func 对象,
		// 因此这里只能确认 Outer 上有一个 Name, 且它的路由注解来自 Outer 自己的声明.
		if name.Recv == nil || name.Recv.Name != "Outer" {
			t.Errorf("Outer.Name recv = %+v, want Outer (shadowed, not promoted)", name.Recv)
		}
	})

	t.Run("提升方法保留原注解", func(t *testing.T) {
		outer := q.FindType("Outer")
		if outer == nil {
			t.Fatal("expected to find Outer type")
		}
		count := findEmbeddedMethod(t, outer, "Count")
		if !HasAnnotation(count.Doc, "GET") {
			t.Errorf("promoted Count lost its @GET annotation: %+v", count.Doc)
		}
		label := findEmbeddedMethod(t, outer, "Label")
		if !HasAnnotation(label.Doc, "GET") {
			t.Errorf("promoted Label lost its @GET annotation: %+v", label.Doc)
		}
	})

	t.Run("非泛型结构体嵌入泛型基础", func(t *testing.T) {
		plain := q.FindType("Plain")
		if plain == nil {
			t.Fatal("expected to find Plain type")
		}
		got := methodNames(plain.Methods)
		want := []string{"Count", "Describe", "Name"}
		if len(got) != len(want) {
			t.Fatalf("Plain methods = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Plain methods = %v, want %v", got, want)
			}
		}
	})

	t.Run("提升方法经 JSON 往返后仍在", func(t *testing.T) {
		outputFile := filepath.Join(t.TempDir(), ".astp.json")
		if err := NewGenerator().Generate(absDir, outputFile); err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		reloaded, err := Load(outputFile)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		outer := NewQuery(reloaded).FindType("Outer")
		if outer == nil {
			t.Fatal("expected to find Outer type after round-trip")
		}
		if got := methodNames(outer.Methods); len(got) != 4 {
			t.Fatalf("Outer methods after round-trip = %v, want 4 entries", got)
		}
	})
}
