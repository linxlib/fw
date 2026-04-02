package astp

type Project struct {
	Packages map[string]*Package `json:"packages"`
}

type Package struct {
	Name      string            `json:"name"`
	Path      string            `json:"path"`
	Types     map[string]*Type  `json:"types"`
	Functions map[string]*Func  `json:"functions"`
	Variables map[string]*Var   `json:"variables"`
	Constants map[string]*Const `json:"constants"`
	Enums     map[string]*Enum  `json:"enums,omitempty"`
}

type TypeKind string

const (
	KindStruct    TypeKind = "struct"
	KindInterface TypeKind = "interface"
	KindAlias     TypeKind = "alias"
	KindBasic     TypeKind = "basic"
	KindSlice     TypeKind = "slice"
	KindMap       TypeKind = "map"
	KindPointer   TypeKind = "pointer"
	KindFunc      TypeKind = "func"
	KindEnum      TypeKind = "enum"
)

type Type struct {
	Name       string        `json:"name"`
	PkgPath    string        `json:"pkg_path"`
	Kind       TypeKind      `json:"kind"`
	Fields     []*Field      `json:"fields,omitempty"`
	Methods    []*Func       `json:"methods,omitempty"`
	Embedded   []*TypeRef    `json:"embedded,omitempty"`
	Implements []string      `json:"implements,omitempty"`
	Generic    *GenericSpec  `json:"generic,omitempty"`
	AliasOf    *TypeRef      `json:"alias_of,omitempty"`
	KeyType    *TypeRef      `json:"key_type,omitempty"`
	ElemType   *TypeRef      `json:"elem_type,omitempty"`
	Doc        *CommentGroup `json:"doc,omitempty"`
}

type TypeRef struct {
	Name     string      `json:"name,omitempty"`
	PkgPath  string      `json:"pkg_path,omitempty"`
	Kind     TypeKind    `json:"kind"`
	Generic  *GenericArg `json:"generic,omitempty"`
	KeyType  *TypeRef    `json:"key_type,omitempty"`
	ElemType *TypeRef    `json:"elem_type,omitempty"`
}

type GenericSpec struct {
	Params []*GenericParam `json:"params"`
}

type GenericParam struct {
	Name        string     `json:"name"`
	Constraints []*TypeRef `json:"constraints,omitempty"`
}

type GenericArg struct {
	Args []*TypeRef `json:"args"`
}

type Field struct {
	Name     string            `json:"name"`
	Type     *TypeRef          `json:"type"`
	Tag      map[string]string `json:"tag,omitempty"`
	Doc      *CommentGroup     `json:"doc,omitempty"`
	Embedded bool              `json:"embedded"`
}

type Func struct {
	Name       string        `json:"name"`
	PkgPath    string        `json:"pkg_path"`
	Recv       *TypeRef      `json:"recv,omitempty"`
	Params     []*Param      `json:"params,omitempty"`
	Results    []*Param      `json:"results,omitempty"`
	Generic    *GenericSpec  `json:"generic,omitempty"`
	Doc        *CommentGroup `json:"doc,omitempty"`
	Signatures []string      `json:"signatures,omitempty"`
}

type Param struct {
	Name string   `json:"name"`
	Type *TypeRef `json:"type"`
}

type Var struct {
	Name    string        `json:"name"`
	PkgPath string        `json:"pkg_path"`
	Type    *TypeRef      `json:"type"`
	Doc     *CommentGroup `json:"doc,omitempty"`
}

type Const struct {
	Name    string        `json:"name"`
	PkgPath string        `json:"pkg_path"`
	Type    *TypeRef      `json:"type,omitempty"`
	Value   string        `json:"value,omitempty"`
	Doc     *CommentGroup `json:"doc,omitempty"`
}

type Enum struct {
	Name    string        `json:"name"`
	PkgPath string        `json:"pkg_path"`
	Type    *TypeRef      `json:"type,omitempty"`
	Values  []*EnumValue  `json:"values"`
	Doc     *CommentGroup `json:"doc,omitempty"`
}

type EnumValue struct {
	Name  string        `json:"name"`
	Value string        `json:"value,omitempty"`
	Doc   *CommentGroup `json:"doc,omitempty"`
}

type CommentGroup struct {
	List              []*Comment    `json:"list,omitempty"`
	Annotations       []string      `json:"annotations,omitempty"`
	ParsedAnnotations []*Annotation `json:"parsed_annotations,omitempty"`
}

type Annotation struct {
	Name string            `json:"name"`
	Raw  string            `json:"raw,omitempty"`
	Args []string          `json:"args,omitempty"`
	KV   map[string]string `json:"kv,omitempty"`
}

type Comment struct {
	Text string `json:"text"`
}
