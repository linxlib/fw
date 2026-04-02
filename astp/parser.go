package astp

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

type ModuleInfo struct {
	Path string
	Dir  string
}

func parseGoMod(dir string) (*ModuleInfo, error) {
	goModPath := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("go.mod not found in %s", dir)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimPrefix(line, "module ")
			modulePath = strings.TrimSpace(modulePath)
			return &ModuleInfo{
				Path: modulePath,
				Dir:  dir,
			}, nil
		}
	}

	return nil, fmt.Errorf("module directive not found in go.mod")
}

func findGoMod(dir string) (*ModuleInfo, error) {
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return parseGoMod(dir)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("go.mod not found in %s or any parent directory", dir)
		}
		dir = parent
	}
}

type Parser struct {
	fset         *token.FileSet
	pkgs         map[string]*types.Package
	parsedFiles  map[string]bool
	project      *Project
	ExportedOnly bool
	modulePath   string
	moduleDir    string
}

func NewParser() *Parser {
	return &Parser{
		fset:        token.NewFileSet(),
		pkgs:        make(map[string]*types.Package),
		parsedFiles: make(map[string]bool),
		project: &Project{
			Packages: make(map[string]*Package),
		},
	}
}

func (p *Parser) Parse(dir string) (*Project, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("get absolute path: %w", err)
	}

	pkgInfo, files, pkgPath, err := p.loadPackage(absDir)
	if err != nil {
		return nil, fmt.Errorf("load package: %w", err)
	}

	if _, exists := p.project.Packages[pkgPath]; !exists {
		p.project.Packages[pkgPath] = &Package{
			Name:      pkgInfo.Name(),
			Path:      pkgPath,
			Types:     make(map[string]*Type),
			Functions: make(map[string]*Func),
			Variables: make(map[string]*Var),
			Constants: make(map[string]*Const),
		}
	}

	for _, file := range files {
		filePath := p.fset.Position(file.Pos()).Filename
		if p.parsedFiles[filePath] {
			continue
		}
		p.parsedFiles[filePath] = true
		p.parseFile(pkgInfo, file, pkgPath)
	}

	return p.project, nil
}

func (p *Parser) loadPackage(dir string) (*types.Package, []*ast.File, string, error) {
	ctx := build.Default
	ctx.CgoEnabled = false

	bpkg, err := ctx.ImportDir(dir, 0)
	if err != nil {
		return nil, nil, "", err
	}

	var files []*ast.File
	for _, goFile := range bpkg.GoFiles {
		filePath := filepath.Join(dir, goFile)
		f, err := parser.ParseFile(p.fset, filePath, nil, parser.ParseComments|parser.AllErrors)
		if err != nil {
			continue
		}
		files = append(files, f)
	}

	if len(files) == 0 {
		return nil, nil, "", nil
	}

	pkgPath := bpkg.ImportPath
	if p.modulePath != "" && p.moduleDir != "" {
		relPath, err := filepath.Rel(p.moduleDir, dir)
		if err == nil {
			relPath = filepath.ToSlash(relPath)
			if relPath == "." {
				pkgPath = p.modulePath
			} else {
				pkgPath = p.modulePath + "/" + relPath
			}
		}
	}

	tpkg := types.NewPackage(pkgPath, bpkg.Name)
	p.pkgs[pkgPath] = tpkg

	return tpkg, files, pkgPath, nil
}

func (p *Parser) parseFile(pkg *types.Package, file *ast.File, pkgPath string) {
	pkgInfo := p.project.Packages[pkgPath]

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			p.parseGenDecl(pkgInfo, d)
		case *ast.FuncDecl:
			p.parseFuncDecl(pkgInfo, d)
		}
	}
}

func (p *Parser) parseGenDecl(pkgInfo *Package, decl *ast.GenDecl) {
	if decl.Tok == token.CONST {
		p.parseConstDecl(pkgInfo, decl)
		return
	}

	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if p.ExportedOnly && !isExported(s.Name.Name) {
				continue
			}
			t := p.parseTypeSpec(s, decl.Doc, pkgInfo.Path)
			if t != nil {
				pkgInfo.Types[t.Name] = t
			}
		case *ast.ValueSpec:
			for _, name := range s.Names {
				if p.ExportedOnly && !isExported(name.Name) {
					continue
				}
				v := &Var{
					Name:    name.Name,
					PkgPath: pkgInfo.Path,
					Type:    p.parseTypeRef(s.Type),
					Doc:     parseDoc(s.Doc),
				}
				pkgInfo.Variables[name.Name] = v
			}
		}
	}
}

func (p *Parser) parseConstDecl(pkgInfo *Package, decl *ast.GenDecl) {
	var enumType *TypeRef
	var enumValues []*EnumValue
	var enumDoc *CommentGroup
	var enumTypeName string
	var lastType *TypeRef

	for _, spec := range decl.Specs {
		s, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		currentType := p.parseTypeRef(s.Type)
		if currentType != nil && currentType.Name != "" {
			if _, exists := pkgInfo.Types[currentType.Name]; exists {
				if enumType == nil || currentType.Name != enumTypeName {
					if enumType != nil && len(enumValues) > 0 {
						p.saveEnum(pkgInfo, enumTypeName, enumType, enumValues, enumDoc)
					}
					enumType = currentType
					enumTypeName = currentType.Name
					enumDoc = parseDoc(decl.Doc)
					if enumDoc == nil {
						enumDoc = parseDoc(s.Doc)
					}
					enumValues = nil
				}
			}
			lastType = currentType
		}

		for i, name := range s.Names {
			if p.ExportedOnly && !isExported(name.Name) {
				continue
			}

			c := &Const{
				Name:    name.Name,
				PkgPath: pkgInfo.Path,
				Doc:     parseDoc(s.Doc),
			}
			if i < len(s.Values) {
				c.Value = exprToString(s.Values[i])
			}
			if s.Type != nil {
				c.Type = p.parseTypeRef(s.Type)
			} else if lastType != nil {
				c.Type = lastType
			}
			pkgInfo.Constants[name.Name] = c

			if enumType != nil {
				value := ""
				if i < len(s.Values) {
					value = exprToString(s.Values[i])
				}
				enumValues = append(enumValues, &EnumValue{
					Name:  name.Name,
					Value: value,
					Doc:   parseDoc(s.Doc),
				})
			}
		}
	}

	if enumType != nil && len(enumValues) > 0 {
		p.saveEnum(pkgInfo, enumTypeName, enumType, enumValues, enumDoc)
	}
}

func (p *Parser) saveEnum(pkgInfo *Package, name string, typ *TypeRef, values []*EnumValue, doc *CommentGroup) {
	if pkgInfo.Enums == nil {
		pkgInfo.Enums = make(map[string]*Enum)
	}

	enum := &Enum{
		Name:    name,
		PkgPath: pkgInfo.Path,
		Type:    typ,
		Values:  values,
		Doc:     doc,
	}
	pkgInfo.Enums[name] = enum

	if t, exists := pkgInfo.Types[name]; exists {
		t.Kind = KindEnum
	}
}

func (p *Parser) parseTypeSpec(spec *ast.TypeSpec, doc *ast.CommentGroup, pkgPath string) *Type {
	t := &Type{
		Name:    spec.Name.Name,
		PkgPath: pkgPath,
		Doc:     parseDoc(doc),
	}

	if spec.Doc != nil && t.Doc == nil {
		t.Doc = parseDoc(spec.Doc)
	}

	if spec.TypeParams != nil {
		t.Generic = &GenericSpec{}
		for _, param := range spec.TypeParams.List {
			for _, name := range param.Names {
				gp := &GenericParam{Name: name.Name}
				if param.Type != nil {
					gp.Constraints = append(gp.Constraints, p.parseTypeRef(param.Type))
				}
				t.Generic.Params = append(t.Generic.Params, gp)
			}
		}
	}

	switch st := spec.Type.(type) {
	case *ast.StructType:
		t.Kind = KindStruct
		t.Fields = p.parseFields(st.Fields)
	case *ast.InterfaceType:
		t.Kind = KindInterface
		t.Methods = p.parseInterfaceMethods(st.Methods, pkgPath)
	case *ast.Ident:
		t.Kind = KindAlias
		t.AliasOf = &TypeRef{Name: st.Name, Kind: KindBasic}
	case *ast.SelectorExpr:
		t.Kind = KindAlias
		t.AliasOf = &TypeRef{
			Name:    st.Sel.Name,
			PkgPath: p.getPackagePath(st.X),
		}
	case *ast.ArrayType:
		t.Kind = KindSlice
		t.ElemType = p.parseTypeRef(st.Elt)
	case *ast.MapType:
		t.Kind = KindMap
		t.KeyType = p.parseTypeRef(st.Key)
		t.ElemType = p.parseTypeRef(st.Value)
	case *ast.StarExpr:
		t.Kind = KindPointer
		t.ElemType = p.parseTypeRef(st.X)
	case *ast.FuncType:
		t.Kind = KindFunc
		t.Methods = []*Func{{Params: p.parseParams(st.Params), Results: p.parseParams(st.Results)}}
	}

	return t
}

func (p *Parser) parseFields(fields *ast.FieldList) []*Field {
	var result []*Field
	if fields == nil {
		return result
	}

	for _, f := range fields.List {
		ft := p.parseTypeRef(f.Type)
		doc := parseDoc(f.Doc)
		if doc == nil {
			doc = parseDoc(f.Comment)
		}

		if len(f.Names) == 0 {
			result = append(result, &Field{
				Type:     ft,
				Doc:      doc,
				Embedded: true,
			})
		} else {
			for _, name := range f.Names {
				if p.ExportedOnly && !isExported(name.Name) {
					continue
				}
				field := &Field{
					Name:     name.Name,
					Type:     ft,
					Doc:      doc,
					Embedded: false,
				}
				if f.Tag != nil {
					field.Tag = parseTag(f.Tag.Value)
				}
				result = append(result, field)
			}
		}
	}

	return result
}

func (p *Parser) parseInterfaceMethods(methods *ast.FieldList, pkgPath string) []*Func {
	var result []*Func
	if methods == nil {
		return result
	}

	for _, m := range methods.List {
		doc := parseDoc(m.Doc)

		if len(m.Names) == 0 {
			if se, ok := m.Type.(*ast.SelectorExpr); ok {
				result = append(result, &Func{
					Name:    se.Sel.Name,
					PkgPath: pkgPath,
					Doc:     doc,
				})
			}
			continue
		}

		for _, name := range m.Names {
			fn := &Func{
				Name:    name.Name,
				PkgPath: pkgPath,
				Doc:     doc,
			}
			if ft, ok := m.Type.(*ast.FuncType); ok {
				fn.Params = p.parseParams(ft.Params)
				fn.Results = p.parseParams(ft.Results)
			}
			result = append(result, fn)
		}
	}

	return result
}

func (p *Parser) parseFuncDecl(pkgInfo *Package, decl *ast.FuncDecl) {
	if p.ExportedOnly && !isExported(decl.Name.Name) {
		return
	}

	fn := &Func{
		Name:    decl.Name.Name,
		PkgPath: pkgInfo.Path,
		Doc:     parseDoc(decl.Doc),
	}

	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		recv := decl.Recv.List[0]
		fn.Recv = p.parseTypeRef(recv.Type)
	}

	if decl.Type.TypeParams != nil {
		fn.Generic = &GenericSpec{}
		for _, param := range decl.Type.TypeParams.List {
			for _, name := range param.Names {
				gp := &GenericParam{Name: name.Name}
				fn.Generic.Params = append(fn.Generic.Params, gp)
			}
		}
	}

	fn.Params = p.parseParams(decl.Type.Params)
	fn.Results = p.parseParams(decl.Type.Results)

	if fn.Recv != nil {
		if t, ok := pkgInfo.Types[fn.Recv.Name]; ok {
			t.Methods = append(t.Methods, fn)
		}
	} else {
		pkgInfo.Functions[fn.Name] = fn
	}
}

func (p *Parser) parseParams(params *ast.FieldList) []*Param {
	var result []*Param
	if params == nil {
		return result
	}

	for _, param := range params.List {
		pt := p.parseTypeRef(param.Type)
		if len(param.Names) == 0 {
			result = append(result, &Param{Type: pt})
		} else {
			for _, name := range param.Names {
				result = append(result, &Param{Name: name.Name, Type: pt})
			}
		}
	}

	return result
}

func (p *Parser) parseTypeRef(expr ast.Expr) *TypeRef {
	if expr == nil {
		return nil
	}

	switch t := expr.(type) {
	case *ast.Ident:
		kind := KindBasic
		if !isBasicType(t.Name) {
			kind = KindStruct
		}
		return &TypeRef{Name: t.Name, Kind: kind}
	case *ast.SelectorExpr:
		return &TypeRef{
			Name:    t.Sel.Name,
			PkgPath: p.getPackagePath(t.X),
			Kind:    KindStruct,
		}
	case *ast.StarExpr:
		ref := p.parseTypeRef(t.X)
		ref.Kind = KindPointer
		return ref
	case *ast.ArrayType:
		return &TypeRef{
			Kind:     KindSlice,
			ElemType: p.parseTypeRef(t.Elt),
		}
	case *ast.MapType:
		return &TypeRef{
			Kind:     KindMap,
			KeyType:  p.parseTypeRef(t.Key),
			ElemType: p.parseTypeRef(t.Value),
		}
	case *ast.InterfaceType:
		return &TypeRef{Kind: KindInterface}
	case *ast.FuncType:
		return &TypeRef{Kind: KindFunc}
	case *ast.IndexExpr:
		ref := p.parseTypeRef(t.X)
		ref.Generic = &GenericArg{
			Args: []*TypeRef{p.parseTypeRef(t.Index)},
		}
		return ref
	case *ast.IndexListExpr:
		ref := p.parseTypeRef(t.X)
		ref.Generic = &GenericArg{}
		for _, idx := range t.Indices {
			ref.Generic.Args = append(ref.Generic.Args, p.parseTypeRef(idx))
		}
		return ref
	default:
		return &TypeRef{Kind: KindBasic}
	}
}

func (p *Parser) getPackagePath(expr ast.Expr) string {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func isBasicType(name string) bool {
	switch name {
	case "bool", "byte", "complex64", "complex128",
		"error", "float32", "float64",
		"int", "int8", "int16", "int32", "int64",
		"rune", "string",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return true
	default:
		return false
	}
}

func isExported(name string) bool {
	if len(name) == 0 {
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.BasicLit:
		return t.Value
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}

func (p *Parser) ParseProject(rootDir string) (*Project, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("get absolute path: %w", err)
	}

	moduleInfo, err := findGoMod(absRoot)
	if err != nil {
		return nil, fmt.Errorf("find go.mod: %w", err)
	}

	p.modulePath = moduleInfo.Path
	p.moduleDir = moduleInfo.Dir

	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}

		base := filepath.Base(path)
		if base == "vendor" || base == "node_modules" || strings.HasPrefix(base, ".") {
			return filepath.SkipDir
		}

		goFiles, _ := filepath.Glob(filepath.Join(path, "*.go"))
		if len(goFiles) > 0 {
			_, err := p.Parse(path)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk project: %w", err)
	}

	return p.project, nil
}
