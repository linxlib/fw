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

	// 嵌入字段的方法提升必须先于类型参数标记: 提升上来的方法也要参与作用域计算.
	p.finalizeEmbeddedMethods(pkgPath)
	p.finalizeTypeParams(pkgPath)
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

	t.Generic = p.parseGenericSpec(spec.TypeParams)

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

	// 函数/方法自身声明的类型参数：自由函数（Go 1.18）与方法（Go 1.27 泛型方法）共用。
	// 方法自身类型参数存于 Func.Generic；接收器上的结构体类型实参存于 Func.Recv.Generic。
	fn.Generic = p.parseGenericSpec(decl.Type.TypeParams)

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

// parseGenericSpec 解析声明头的类型参数列表。
//
// 结构体（Go 1.18 类型泛型）、自由函数（Go 1.18 函数泛型）与方法（Go 1.27 方法泛型）
// 共用同一套解析，保证三者的 GenericParam.Constraints 行为一致。
// fields 为 nil 或没有条目时返回 nil，因此「没有类型参数」与「有类型参数」可区分。
func (p *Parser) parseGenericSpec(fields *ast.FieldList) *GenericSpec {
	if fields == nil || len(fields.List) == 0 {
		return nil
	}

	spec := &GenericSpec{}
	for _, param := range fields.List {
		for _, name := range param.Names {
			gp := &GenericParam{Name: name.Name}
			if param.Type != nil {
				gp.Constraints = p.parseTypeParamConstraints(param.Type)
			}
			spec.Params = append(spec.Params, gp)
		}
	}
	return spec
}

// parseTypeParamConstraints 解析单个类型参数的约束表达式，返回被约束类型的引用列表。
//
// 支持的写法（均为 go1.27 实测 AST 形态）：
//
//	T any                        -> Ident(any)
//	T comparable                 -> Ident(comparable)
//	T ~U                         -> UnaryExpr(~)
//	T int | string               -> BinaryExpr(|)
//	T (int | string)             -> ParenExpr
//	T interface{ ~int | string } -> InterfaceType
//
// `~` 运算符本身不落库：`~T` 与 `T` 都记录为对被约束类型的引用，
// 这与 GenericParam.Constraints []*TypeRef 的既有形状一致，无需新增字段。
func (p *Parser) parseTypeParamConstraints(expr ast.Expr) []*TypeRef {
	var out []*TypeRef

	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		switch t := e.(type) {
		case nil:
			return
		case *ast.ParenExpr:
			walk(t.X)
			return
		case *ast.BinaryExpr:
			// 联合约束 A|B（以及 A|B|C 的左结合链）
			if t.Op == token.OR {
				walk(t.X)
				walk(t.Y)
				return
			}
		case *ast.UnaryExpr:
			// ~T：约束为 T 的底层类型集合，保留 T 的引用
			if t.Op == token.TILDE {
				walk(t.X)
				return
			}
		case *ast.InterfaceType:
			// interface{ ~int | string }：接口字面量的元素就是并集的各个分支
			if t.Methods != nil {
				for _, m := range t.Methods.List {
					if m != nil {
						walk(m.Type)
					}
				}
				return
			}
		}

		if ref := p.parseTypeRef(e); ref != nil {
			out = append(out, ref)
		}
	}

	walk(expr)
	return out
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
	case *ast.ParenExpr:
		// 括号包裹的类型表达式（如类型参数约束里的 (int | string)），降级为内部类型
		return p.parseTypeRef(t.X)
	case *ast.UnaryExpr:
		// ~T：约束为 T 的底层类型集合，保留 T 的引用（~ 运算符本身不落库）
		if t.Op == token.TILDE {
			return p.parseTypeRef(t.X)
		}
		return &TypeRef{Kind: KindBasic}
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

// finalizeEmbeddedMethods 把嵌入字段所在类型的方法提升到外层结构体上。
//
// Go 的方法提升规则：外层结构体自动拥有其嵌入字段（含嵌入字段的嵌入字段）的方法；
// 外层自身显式声明的方法优先（遮蔽）。泛型基础控制器的能力正是通过这条规则传递的：
//
//	type BaseController[T any] struct{ ... }
//	func (c *BaseController[T]) Create(...) {}
//
//	type UserController struct {
//	    BaseController[User]   // Create 等方法被提升到 UserController 上
//	}
//
// 提升上来的方法与原方法共享同一个 *Func（只读），其 Recv 仍指向嵌入类型本身。
// 目前只处理同包内可解析的嵌入类型；跨包嵌入类型的方法提升留作后续项。
func (p *Parser) finalizeEmbeddedMethods(pkgPaths ...string) {
	if p.project == nil {
		return
	}

	targets := p.project.Packages
	if len(pkgPaths) > 0 {
		targets = make(map[string]*Package, len(pkgPaths))
		for _, path := range pkgPaths {
			if pkg, ok := p.project.Packages[path]; ok {
				targets[path] = pkg
			}
		}
	}

	for _, pkg := range targets {
		if pkg == nil {
			continue
		}
		for _, t := range pkg.Types {
			if t == nil || t.Kind != KindStruct || len(t.Fields) == 0 {
				continue
			}

			// 已占用的名字：外层自身显式声明的方法优先，同深度先到先得。
			taken := make(map[string]bool, len(t.Methods))
			for _, m := range t.Methods {
				if m != nil {
					taken[m.Name] = true
				}
			}

			var promoted []*Func
			visited := map[string]bool{t.Name: true}
			p.collectEmbeddedMethods(pkg, t, visited, taken, &promoted)
			if len(promoted) > 0 {
				t.Methods = append(t.Methods, promoted...)
			}
		}
	}
}

// collectEmbeddedMethods 深度优先遍历 t 的嵌入字段，把可解析嵌入类型的方法收集到 out。
// taken 记录已被占用的方法名（外层显式方法 + 更浅深度已收集的方法），visited 防止循环嵌入。
func (p *Parser) collectEmbeddedMethods(pkg *Package, t *Type, visited, taken map[string]bool, out *[]*Func) {
	for _, f := range t.Fields {
		if f == nil || !f.Embedded || f.Type == nil || f.Type.Name == "" {
			continue
		}
		name := f.Type.Name
		if visited[name] {
			continue
		}
		visited[name] = true

		emb, ok := pkg.Types[name]
		if !ok || emb == nil || emb.Kind != KindStruct {
			continue
		}
		for _, m := range emb.Methods {
			if m == nil || taken[m.Name] {
				continue
			}
			taken[m.Name] = true
			*out = append(*out, m)
		}
		// 嵌入类型自身也是通过嵌入获得的方法，继续向下展开。
		p.collectEmbeddedMethods(pkg, emb, visited, taken, out)
	}
}

// finalizeTypeParams 在全部声明解析完成后执行一遍后处理：
// 把函数/方法签名中「直接以类型参数为类型」的 TypeRef 标记为 KindTypeParam。
//
// 之所以放到末尾而不是在 parseFuncDecl 里内联做，是因为方法的接收器类型实参
// （如 *Stack[T] 里的 T）需要知道 Stack 自身声明的类型参数，而方法在源码中的位置
// 可能早于结构体声明。放在末尾则无论声明顺序如何都能拿到完整的 pkg.Types。
//
// 可见类型参数名集合 = 该 Func 自身声明的类型参数 ∪（方法）接收器基类型声明的类型参数。
// 方法必与其接收器类型同包，因此这一步是精确的。
//
// 该函数幂等：已标记的引用再次标记结果不变，所以重复调用是安全的。
//
// pkgPaths 限定只处理这些包；不传则处理整个项目。Parse 每次只解析一个目录，
// 因此按包调用即可（方法必与接收器类型同包，按包处理不丢信息），
// 避免 ParseProject 逐目录解析时对整个项目反复全量扫描。
func (p *Parser) finalizeTypeParams(pkgPaths ...string) {
	if p.project == nil {
		return
	}

	targets := p.project.Packages
	if len(pkgPaths) > 0 {
		targets = make(map[string]*Package, len(pkgPaths))
		for _, path := range pkgPaths {
			if pkg, ok := p.project.Packages[path]; ok {
				targets[path] = pkg
			}
		}
	}

	for _, pkg := range targets {
		if pkg == nil {
			continue
		}

		for _, fn := range pkg.Functions {
			if fn == nil {
				continue
			}
			scope := p.typeParamScope(pkg, fn)
			if len(scope) == 0 {
				continue
			}
			markTypeParamRefs(fn.Params, scope)
			markTypeParamRefs(fn.Results, scope)
			markTypeParamRef(fn.Recv, scope)
		}

		for _, typ := range pkg.Types {
			if typ == nil {
				continue
			}
			for _, m := range typ.Methods {
				if m == nil {
					continue
				}
				scope := p.typeParamScope(pkg, m)
				if len(scope) == 0 {
					continue
				}
				markTypeParamRefs(m.Params, scope)
				markTypeParamRefs(m.Results, scope)
				markTypeParamRef(m.Recv, scope)
			}
		}
	}
}

// typeParamScope 收集一个函数/方法签名内可见的类型参数名。
func (p *Parser) typeParamScope(pkg *Package, fn *Func) map[string]bool {
	if fn == nil {
		return nil
	}

	scope := make(map[string]bool)
	for _, gp := range genericParamNames(fn.Generic) {
		scope[gp] = true
	}

	// 方法：接收器基类型声明的类型参数同样在签名内可见。
	// 例如 func (s *Stack[T]) Pop() T，这里的 T 来自 Stack 自身的声明。
	if fn.Recv != nil && fn.Recv.Name != "" && pkg != nil {
		if base, ok := pkg.Types[fn.Recv.Name]; ok {
			for _, gp := range genericParamNames(base.Generic) {
				scope[gp] = true
			}
		}
	}

	return scope
}

func genericParamNames(spec *GenericSpec) []string {
	if spec == nil {
		return nil
	}
	var names []string
	for _, gp := range spec.Params {
		if gp != nil && gp.Name != "" {
			names = append(names, gp.Name)
		}
	}
	return names
}

// markTypeParamRefs 把一个参数/返回值列表里直接以类型参数为类型的引用标记出来。
//
// 标记规则刻意保守：
//   - 只改 Kind == KindStruct 且 PkgPath == "" 的引用，也就是「同包内的裸名字」；
//     带 pkg_path 的跨包引用（demo.TestStruct）不可能是类型参数。
//   - *V 这类外层为指针的引用保持 KindPointer：指针形状对调用方更有价值，
//     因此不改写（代价是这类引用仍按名字解析）。
//   - []T 的 elem_type、map[K]V 的 key_type/elem_type 会被递归标记。
func markTypeParamRefs(params []*Param, scope map[string]bool) {
	for _, param := range params {
		if param == nil {
			continue
		}
		markTypeParamRef(param.Type, scope)
	}
}

func markTypeParamRef(ref *TypeRef, scope map[string]bool) {
	if ref == nil || len(scope) == 0 {
		return
	}

	if ref.Kind == KindStruct && ref.PkgPath == "" && scope[ref.Name] {
		ref.Kind = KindTypeParam
	}

	if ref.Generic != nil {
		for _, arg := range ref.Generic.Args {
			markTypeParamRef(arg, scope)
		}
	}
	markTypeParamRef(ref.KeyType, scope)
	markTypeParamRef(ref.ElemType, scope)
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

	p.finalizeEmbeddedMethods()
	p.finalizeTypeParams()

	return p.project, nil
}
