# astp

Golang AST 语法树解析库，用于解析 Go 源代码生成结构化描述数据，支持在运行时查询代码信息（包括注释、Tag 等运行时不存在的信息）。

## 特性

- 解析结构体、接口、函数、变量、常量、枚举
- 支持泛型类型
- 支持 Go 1.27 方法泛型（方法自身声明的类型参数）
- 支持内嵌字段的方法提升，以及泛型实参绑定与展开
- 解析 struct tag
- 解析注释中的 `@xxx` 注解
- 支持类型引用解析
- 支持 go.mod 模块路径解析
- JSON 格式输出

## 安装

```bash
go get github.com/linxlib/astp
```

## 快速开始

### 1. 生成描述数据

#### 命令行方式

```bash
# 安装命令行工具
go install github.com/linxlib/astp/cmd/astp@latest

# 解析项目（包含私有成员）
astp /path/to/project

# 仅解析导出成员
astp -exported /path/to/project

# 指定输出文件
astp -o custom.json /path/to/project
```

#### 代码方式

```go
package main

import "github.com/linxlib/astp"

func main() {
    // 创建生成器
    g := astp.NewGenerator()
    
    // 设置仅解析导出成员
    g.ExportedOnly = true
    
    // 生成描述数据
    err := g.Generate("/path/to/project", "")
    if err != nil {
        panic(err)
    }
    // 默认输出到 /path/to/project/.astp.json
}
```

### 2. 加载描述数据

```go
package main

import "github.com/linxlib/astp"

func main() {
    // 从目录加载（查找 .astp.json）
    project, err := astp.Load("/path/to/project")
    if err != nil {
        panic(err)
    }
    
    // 或从指定文件加载
    project, err = astp.Load("/path/to/project/custom.json")
    if err != nil {
        panic(err)
    }
    
    // 创建查询器
    query := astp.NewQuery(project)
}
```

## 查询 API

### 类型查询

```go
// 按名称查找类型
user := query.FindType("User")

// 按包路径和名称查找
user := query.FindTypeByPath("github.com/myproject/models", "User")

// 获取结构体
user := query.GetStruct("User")

// 获取接口
reader := query.GetInterface("Reader")

// 列出包下所有类型
types := query.ListTypes("github.com/myproject/models")
```

### 函数查询

```go
// 按名称查找函数
fn := query.FindFunc("CreateUser")

// 获取方法的参数和返回值类型
params, results := query.GetMethodParams(method)
```

### 类型引用解析

当函数参数或字段类型是结构体时，可以解析出完整的类型信息：

```go
// 解析 TypeRef 为完整类型
resolvedType := query.ResolveTypeRef(param.Type)

// 解析参数类型
resolvedType := query.ResolveParamType(param)

// 解析字段类型
resolvedType := query.ResolveFieldType(field)
```

示例：

```go
// 假设有方法: func (u *User) Copy() *User
method := user.Methods[0]

// 获取返回值类型
returnType := method.Results[0].Type
// returnType.Name = "User", 但没有字段信息

// 解析完整类型
resolved := query.ResolveTypeRef(returnType)
// resolved.Name = "User"
// resolved.Fields = [...完整的字段列表]
// resolved.Methods = [...完整的方法列表]
```

### 注解查询

代码中的 `@xxx` 注解会被自动解析：

```go
// @Service
// @Route("/api")
type UserService struct {}

// 查找带有 @Service 注解的类型
types := query.FindTypesByAnnotation("Service")

// 查找带有 @GET 注解的函数/方法
funcs := query.FindFuncsByAnnotation("GET")

// 检查类型是否有某个注解
if astp.HasAnnotation(userType.Doc, "Service") {
    // ...
}

// 获取注解值
value := astp.GetAnnotationValue(method.Doc, "Route")  // "/api"
```

### Tag 查询

```go
type User struct {
    Name string `json:"name" db:"user_name"`
}

// 查找带有 json tag 的字段
fields := query.FindByTag("json")

// 查找带有特定值的 tag
fields := query.FindFieldsByTag("json", "name")

// 获取字段 tag 值
tagValue := astp.GetTag(field.Tag, "json")  // "name"

// 检查字段是否有某个 tag
if astp.HasTag(field.Tag, "db") {
    // ...
}
```

### 枚举查询

```go
type Status int

const (
    StatusActive Status = iota
    StatusInactive
    StatusPending
)

// 查找枚举
statusEnum := query.FindEnum("Status")

// 获取枚举值
for _, v := range statusEnum.Values {
    fmt.Printf("%s = %s\n", v.Name, v.Value)
}

// 列出所有枚举
enums := query.ListEnums()

// 获取枚举值列表
values := query.GetEnumValues("Status")
```

## 数据结构

### Project

```go
type Project struct {
    Packages map[string]*Package
}
```

### Package

```go
type Package struct {
    Name      string
    Path      string           // 完整包路径，如 github.com/myproject/models
    Types     map[string]*Type
    Functions map[string]*Func
    Variables map[string]*Var
    Constants map[string]*Const
    Enums     map[string]*Enum
}
```

### Type

```go
type Type struct {
    Name    string        // 类型名称
    PkgPath string        // 包路径
    Kind    TypeKind      // struct, interface, alias, enum 等
    Fields  []*Field      // 结构体字段
    Methods []*Func       // 方法
    Generic *GenericSpec  // 泛型参数
    Doc     *CommentGroup // 文档注释
}
```

### Func

```go
type Func struct {
    Name    string        // 函数/方法名
    PkgPath string
    Recv    *TypeRef      // 接收者（方法）
    Params  []*Param      // 参数
    Results []*Param      // 返回值
    Generic *GenericSpec  // 泛型参数
    Doc     *CommentGroup // 文档注释
}
```

### Field

```go
type Field struct {
    Name     string            // 字段名
    Type     *TypeRef          // 字段类型
    Tag      map[string]string // struct tag
    Doc      *CommentGroup     // 文档注释
    Embedded bool              // 是否嵌入字段
}
```

### Enum

```go
type Enum struct {
    Name    string        // 枚举名称
    PkgPath string
    Type    *TypeRef      // 枚举底层类型
    Values  []*EnumValue  // 枚举值
    Doc     *CommentGroup
}

type EnumValue struct {
    Name  string
    Value string        // 枚举值
    Doc   *CommentGroup
}
```

## 输出文件示例

```json
{
  "packages": {
    "github.com/myproject/models": {
      "name": "models",
      "path": "github.com/myproject/models",
      "types": {
        "User": {
          "name": "User",
          "pkg_path": "github.com/myproject/models",
          "kind": "struct",
          "fields": [
            {
              "name": "Name",
              "type": {"name": "string", "kind": "basic"},
              "tag": {"json": "name"}
            }
          ],
          "methods": [
            {
              "name": "GetName",
              "results": [{"type": {"name": "string", "kind": "basic"}}]
            }
          ]
        }
      },
      "enums": {
        "Status": {
          "name": "Status",
          "type": {"name": "int", "kind": "basic"},
          "values": [
            {"name": "StatusActive", "value": "iota"},
            {"name": "StatusInactive"},
            {"name": "StatusPending"}
          ]
        }
      }
    }
  }
}
```

## 注意事项

1. **go.mod 必须存在**: 解析时会查找 go.mod 获取正确的模块路径，如果不存在会报错中断

2. **私有成员过滤**: 使用 `-exported` 选项或设置 `ExportedOnly = true` 可过滤掉未导出的类型、函数、字段

3. **类型解析**: `TypeRef` 只包含类型引用信息，使用 `ResolveTypeRef()` 可获取完整类型定义

4. **循环引用**: 类型之间的引用通过 `TypeRef` 实现，避免循环引用问题

## 泛型支持

### 数据形态

- `Type.Generic` 与 `Func.Generic` 是 `*GenericSpec`，里面是该方法/类型自身声明的类型参数
- `Func.Recv.Generic` 是 `*GenericArg`，保存接收者结构体的类型实参，例如 `BaseController[models.User]`
- 签名里引用类型参数的位置，`TypeRef.Kind` 为 `KindTypeParam`（`"typeparam"`），此时 `ResolveTypeRef()` 返回 nil，避免误解析到同名具名类型
- 约束支持 `any`、`comparable`、`~T`、`A|B`、`(A|B)` 与 `interface{...}`，`~` 不落库

### 内嵌方法提升

同包内嵌字段的方法会被提升到外层结构体，外层显式方法遮蔽被提升的方法。提升方法与原方法共享
同一个 `*Func`，`Recv` 仍指向嵌入类型。跨包嵌入暂不提升。

### 泛型实参绑定

```go
bind := astp.RecvTypeArgs(q, owner, fn)     // 从嵌入字段链还原 T -> models.User
ref := astp.InstantiateTypeRef(sigRef, bind) // 把签名里的 T 替换成真实类型
fields := astp.InstantiateFields(t.Fields, bind)
```

- `TypeArgBinding` 是 `map[string]*TypeRef`，key 为类型参数名
- `InstantiateTypeRef` 按名递归替换并保留外层形状，`*T` 替换后仍是指针，深度上限 8 以防自引用爆栈
- `RebindTypeArgs` 处理同名不同作用域的情况，例如 `Base.T` 与 `PageSize.T`
- 方法自身声明的类型参数无法静态推断实参，`RecvTypeArgs` 对这类方法返回 nil
