package astp

import (
	"go/ast"
	"os"
	"path/filepath"
	"testing"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		input    string
		key      string
		expected string
	}{
		{`json:"name"`, "json", "name"},
		{`json:"name" db:"user_name"`, "json", "name"},
		{`json:"name" db:"user_name"`, "db", "user_name"},
		{`json:"name,omitempty"`, "json", "name,omitempty"},
	}

	for _, tt := range tests {
		tags := parseTag(tt.input)
		if got := GetTag(tags, tt.key); got != tt.expected {
			t.Errorf("parseTag(%q)[%q] = %q, want %q", tt.input, tt.key, got, tt.expected)
		}
	}
}

func TestParseDoc(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		hasAnnot     string
		wantHasAnnot bool
	}{
		{"simple comment", "// Hello world", "Service", false},
		{"with annotation", "// @Service\n// Some description", "Service", true},
		{"multiple annotations", "// @GET\n// @Path(\"/users\")", "GET", true},
		{"no annotation", "// Regular comment", "POST", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc *CommentGroup
			if tt.input != "" {
				doc = parseDoc(&ast.CommentGroup{
					List: []*ast.Comment{{Text: tt.input}},
				})
			}
			if got := HasAnnotation(doc, tt.hasAnnot); got != tt.wantHasAnnot {
				t.Errorf("HasAnnotation() = %v, want %v", got, tt.wantHasAnnot)
			}
		})
	}
}

func TestParser(t *testing.T) {
	testdataDir := filepath.Join(".", "testdata")
	absDir, err := filepath.Abs(testdataDir)
	if err != nil {
		t.Fatalf("get absolute path: %v", err)
	}

	parser := NewParser()
	project, err := parser.Parse(absDir)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(project.Packages) == 0 {
		t.Error("Expected at least one package")
	}

	for pkgPath, pkg := range project.Packages {
		t.Logf("Package: %s", pkgPath)

		for name, typ := range pkg.Types {
			t.Logf("  Type: %s (kind: %s)", name, typ.Kind)
			if typ.Kind == KindStruct {
				for _, f := range typ.Fields {
					t.Logf("    Field: %s %v", f.Name, f.Type)
				}
			}
		}

		for name, fn := range pkg.Functions {
			t.Logf("  Func: %s", name)
			if fn.Doc != nil {
				t.Logf("    Annotations: %v", fn.Doc.Annotations)
			}
		}
	}
}

func TestGeneratorAndLoader(t *testing.T) {
	testdataDir := filepath.Join(".", "testdata")
	absDir, err := filepath.Abs(testdataDir)
	if err != nil {
		t.Fatalf("get absolute path: %v", err)
	}

	outputFile := filepath.Join(t.TempDir(), ".astp.json")

	gen := NewGenerator()
	err = gen.Generate(absDir, outputFile)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatalf("Output file not created: %s", outputFile)
	}

	loader := NewLoader()
	err = loader.LoadFile(outputFile)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	project := loader.Project()
	if project == nil {
		t.Fatal("Project is nil")
	}

	if len(project.Packages) == 0 {
		t.Error("Expected at least one package")
	}
}

func TestQuery(t *testing.T) {
	testdataDir := filepath.Join(".", "testdata")
	absDir, err := filepath.Abs(testdataDir)
	if err != nil {
		t.Fatalf("get absolute path: %v", err)
	}

	outputFile := filepath.Join(t.TempDir(), ".astp.json")

	gen := NewGenerator()
	err = gen.Generate(absDir, outputFile)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	project, err := Load(outputFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	query := NewQuery(project)

	t.Run("FindType", func(t *testing.T) {
		user := query.FindType("User")
		if user == nil {
			t.Error("Expected to find User type")
		}
		if user != nil && user.Kind != KindStruct {
			t.Errorf("User kind = %v, want %v", user.Kind, KindStruct)
		}
	})

	t.Run("FindType - interface", func(t *testing.T) {
		reader := query.FindType("Reader")
		if reader == nil {
			t.Error("Expected to find Reader type")
		}
		if reader != nil && reader.Kind != KindInterface {
			t.Errorf("Reader kind = %v, want %v", reader.Kind, KindInterface)
		}
	})

	t.Run("FindByTag", func(t *testing.T) {
		fields := query.FindByTag("json")
		if len(fields) == 0 {
			t.Error("Expected to find fields with json tag")
		}
	})

	t.Run("FindByAnnotation", func(t *testing.T) {
		types := query.FindTypesByAnnotation("Service")
		if len(types) == 0 {
			t.Error("Expected to find types with @Service annotation")
		}
	})

	t.Run("FindFuncsByAnnotation", func(t *testing.T) {
		funcs := query.FindFuncsByAnnotation("GET")
		if len(funcs) == 0 {
			t.Error("Expected to find funcs with @GET annotation")
		}
	})

	t.Run("ResolveTypeRef", func(t *testing.T) {
		user := query.FindType("User")
		if user == nil {
			t.Fatal("Expected to find User type")
		}

		var copyMethod *Func
		for _, m := range user.Methods {
			if m.Name == "Copy" {
				copyMethod = m
				break
			}
		}
		if copyMethod == nil {
			t.Fatal("Expected to find Copy method")
		}

		if len(copyMethod.Results) == 0 {
			t.Fatal("Expected Copy method to have return value")
		}

		returnType := copyMethod.Results[0].Type
		if returnType == nil {
			t.Fatal("Expected return type")
		}

		resolvedType := query.ResolveTypeRef(returnType)
		if resolvedType == nil {
			t.Error("Expected to resolve return type")
		} else {
			if resolvedType.Name != "User" {
				t.Errorf("Resolved type name = %s, want User", resolvedType.Name)
			}
			if resolvedType.Kind != KindStruct {
				t.Errorf("Resolved type kind = %v, want struct", resolvedType.Kind)
			}
			t.Logf("Resolved type: %s with %d fields", resolvedType.Name, len(resolvedType.Fields))
		}
	})

	t.Run("GetMethodParams", func(t *testing.T) {
		createFunc := query.FindFunc("CreateUser")
		if createFunc == nil {
			t.Fatal("Expected to find CreateUser function")
		}

		_, resultTypes := query.GetMethodParams(createFunc)
		if len(resultTypes) == 0 {
			t.Error("Expected CreateUser to have return type")
		} else {
			t.Logf("CreateUser returns type: %s", resultTypes[0].Name)
		}
	})

	t.Run("FindEnum", func(t *testing.T) {
		statusEnum := query.FindEnum("Status")
		if statusEnum == nil {
			t.Fatal("Expected to find Status enum")
		}

		if statusEnum.Name != "Status" {
			t.Errorf("Enum name = %s, want Status", statusEnum.Name)
		}

		if len(statusEnum.Values) != 3 {
			t.Errorf("Status enum values count = %d, want 3", len(statusEnum.Values))
		}

		expectedValues := []string{"StatusActive", "StatusInactive", "StatusPending"}
		for i, v := range statusEnum.Values {
			if i < len(expectedValues) && v.Name != expectedValues[i] {
				t.Errorf("Status enum value[%d] = %s, want %s", i, v.Name, expectedValues[i])
			}
		}

		t.Logf("Status enum has %d values: %v", len(statusEnum.Values), statusEnum.Values)
	})

	t.Run("ListEnums", func(t *testing.T) {
		enums := query.ListEnums()
		if len(enums) < 2 {
			t.Errorf("Expected at least 2 enums, got %d", len(enums))
		}

		for _, e := range enums {
			t.Logf("Enum: %s with %d values", e.Name, len(e.Values))
		}
	})
}
