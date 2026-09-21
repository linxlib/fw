# Go 1.27 方法泛型支持计划

> 本文档已按**本项目（`github.com/linxlib/fw/v2`）的 `astp` 包实际情况**重写。
> 早先版本描述的是外部旧版 `astp` 库（`parsers/`、`types/`、`Function.TypeParam`、
> `Receiver.TypeParam`、`parseTypeParamV2`、`internal.IsInternalGenericType` 等），
> 与当前仓库内的扁平单包 `astp/` 已完全不对应，其中的「核心缺口」结论也已失效。

## 背景

Go 1.27 新增**方法泛型（generic methods）**：方法声明可以拥有自己的类型参数列表。例如：

```go
type Stack[T any] struct {
    items []T
}

// Push 是带自身类型参数 V 的泛型方法
func (s *Stack[T]) Push[V any](v V) {}

// Pop 仅使用接收器(结构)的类型参数 T
func (s *Stack[T]) Pop() T { return s.items[0] }
```

一个方法同时拥有**两类**类型参数：

- **接收器类型实参**：`*Stack[T]` 中的 `T`，是所属结构体类型参数在接收器上的实例化，
  落在 `Func.Recv.Generic`（`*GenericArg`）。
- **方法自身类型参数**：`Push[V any]` 中的 `V`，由该方法单独声明，落在 `Func.Generic`
  （`*GenericSpec`）。

接口方法**不允许**声明类型参数（Go 1.27 规格限制，实测 go1.27.1 解析器直接报错
`interface method must have no type parameters`）。

## 本项目 astp 现状（实测结论）

`astp` 是 `fw/v2` 内的扁平单包（`astp/*.go`，无 `parsers/`、`types/` 子包），
类型模型见 `astp/types.go`：`Project` → `Packages` → `Package{Types, Functions, ...}`，
函数与方法共用同一个 `Func` 结构（`Name / PkgPath / Recv / Params / Results / Generic / Doc / Signatures`）。
解析入口是 `astp/parser.go`：

`Parser.ParseProject` → 逐目录 `Parser.Parse` → `loadPackage`（`build.ImportDir` + `parser.ParseFile`）
→ `parseFile` → `parseGenDecl` / `parseFuncDecl` → `parseTypeSpec` / `parseFields` /
`parseParams` / `parseTypeRef`。

关键实测结论（go1.27.1 windows/amd64）：

1. **`go/parser` 侧无需任何改动。** `GOROOT/src/go/parser/parser.go` 的 `parseFuncDecl` 中
   `if p.tok == token.LBRACK { tparams = p.parseTypeParameters() }` 对方法同样成立，
   无 GODEBUG/GOEXPERIMENT 门控，方法类型参数被无条件写入 `ast.FuncDecl.Type.TypeParams`。
   因此 `loadPackage` 里既有的 `parser.ParseFile` 调用已经能产出含方法泛型的 AST。
2. **方法自身类型参数其实已经被解析。** `parseFuncDecl` 现有的
   `if decl.Type.TypeParams != nil { fn.Generic = ... }` 对方法一样生效，
   实测 `func (s *Stack[T]) Push[V any](v V)` 产出
   `func.generic = {"params":[{"name":"V"}]}`。
   早先文档声称的「只读 `decl.Type.Params`/`decl.Type.Results`，从未读 `decl.Type.TypeParams`」
   在本项目并不成立。
3. **真正的缺口有四处**（详见下）。

### 缺口 1：函数/方法的类型参数丢约束

`parseFuncDecl` 复制了 `parseTypeSpec` 的类型参数解析循环，但**没有**写
`GenericParam.Constraints`：

```go
// astp/parser.go parseFuncDecl（现状）
for _, param := range decl.Type.TypeParams.List {
    for _, name := range param.Names {
        gp := &GenericParam{Name: name.Name}   // ← Constraints 丢失
        fn.Generic.Params = append(fn.Generic.Params, gp)
    }
}
```

对比 `parseTypeSpec`（结构体）同样位置是有 `gp.Constraints = append(..., p.parseTypeRef(param.Type))` 的。
结果：`Push[V any]` 的 `V` 与 `Wrap[V ~T]` 的 `V` 在输出上完全一样，
`any` / `comparable` / `~T` / `int | string` 全部消失。对泛型方法而言，
约束是方法自身类型参数最主要的语义信息。

### 缺口 2：约束表达式语法未被支持

`parseTypeRef` 只识别 `Ident / SelectorExpr / StarExpr / ArrayType / MapType /
InterfaceType / FuncType / IndexExpr / IndexListExpr`，其余走 `default` 返回 `{kind:"basic"}`。
实测 AST 形态：

| 写法 | `fl.Type` 实际形态 | 现状 |
| --- | --- | --- |
| `V any` / `V comparable` | `*ast.Ident` | 正常 |
| `V ~T` | `*ast.UnaryExpr`（`token.TILDE`） | → `{kind:"basic"}`，丢失 |
| `V int \| string` | `*ast.BinaryExpr`（`token.OR`） | → `{kind:"basic"}`，丢失 |
| `V interface{ ~int \| string }` | `*ast.InterfaceType` | → `{kind:"interface"}`，丢失内部并集 |

### 缺口 3：类型参数引用与具名类型无法区分

`parseParams` 只做 `parseTypeRef(param.Type)`，不感知外层声明的类型参数。
`Push[V any](v V)` 产出的 `params[0].type` 是 `{"name":"V","kind":"struct"}`，
与「引用了一个叫 V 的结构体」完全同形。
后果：`Query.ResolveTypeRef` 会按 `Kind == KindStruct` 去 `FindType("V")`，
一旦项目里真有一个叫 `V` 的类型就会**误解析**；即使解析不到，
调用方也无法区分「这是一个待定的类型参数」和「这是一个暂时没解析到的类型」。

### 缺口 4：接收器类型实参在签名内未被标记

`Pop() T` 的返回值 `T` 与缺口 3 同病。要正确标记它，必须知道
「`*Stack[T]` 里的 `T` 是 `Stack` 自己声明的类型参数」——这需要拿到接收器基类型的
`Type.Generic.Params`，而方法在源码中的位置可能早于结构体声明，
因此不能简单地按声明顺序内联处理。

## 设计决策

| 决策点 | 选择 |
| --- | --- |
| 方法自身类型参数存储位置 | **复用既有 `Func.Generic`**（`*GenericSpec`）。接收器类型实参保持在 `Func.Recv.Generic`（`*GenericArg`）。两者本就分离，无需新增字段、不改变 JSON schema |
| 如何标记「这是类型参数引用」 | 新增 `TypeKind` 取值 `KindTypeParam`（JSON `"typeparam"`），把签名中直接以类型参数为类型的 `TypeRef.Kind` 置为该值。枚举值新增对旧 `.astp.json` 数据是向后兼容的（读入只是多一个新字符串） |
| 约束解析 | 抽出 `parseGenericSpec` / `parseTypeParamConstraints` 两个函数，`parseTypeSpec` 与 `parseFuncDecl` 共用，消除重复并顺手修掉缺口 1 |
| 约束表达式支持范围 | `any` / `comparable` / 具名类型 / `~T` / `A \| B` / `(A \| B)` / `interface{ A \| B }`。`~` 运算符本身**不**落库，约束以「被约束类型引用列表」保留（与 `GenericParam.Constraints []*TypeRef` 的既有形状一致，无需加字段） |
| 类型参数标记的作用域 | 自身声明的类型参数 ∪（方法）接收器基类型声明的类型参数。在**全部声明解析完之后**统一做一遍后处理（`finalizeTypeParams`），因此与源码声明顺序无关 |
| 是否升级 `go.mod` | **否**（本步骤）。`go/parser` 的产出与 go.mod 声明的语言版本无关，实测 `go 1.25` 下即可解析泛型方法；新 fixture 放在 `astp/testdata/`，go 工具忽略该目录、不参与编译。fw 整体语言版本升级是独立决策 |
| 结构体字段是否也标记 | **否**，本步骤只覆盖函数/方法签名（`Func.Params` / `Func.Results`）。字段侧（`Type.Fields`）作为后续项 |
| 接口方法 | 无需处理。Go 1.27 禁止，`parser.ParseFile` 直接报错，`loadPackage` 对解析失败的文件是 `continue` 跳过 |

## 改动清单

### 1. `astp/types.go`

新增一个 `TypeKind` 取值：

```go
KindTypeParam TypeKind = "typeparam"
```

### 2. `astp/parser.go`

**2.1 抽出共用的类型参数解析（修缺口 1）**

```go
// parseGenericSpec 解析声明头的类型参数列表。
// 结构体（Go 1.18 类型泛型）、自由函数（Go 1.18 函数泛型）与方法（Go 1.27 方法泛型）共用。
func (p *Parser) parseGenericSpec(fields *ast.FieldList) *GenericSpec
```

**2.2 新增约束表达式展开（修缺口 2）**

```go
// parseTypeParamConstraints 解析单个类型参数的约束表达式，
// 递归展开 ParenExpr / BinaryExpr(|) / UnaryExpr(~) / InterfaceType{...}，
// 返回被约束类型的引用列表。
func (p *Parser) parseTypeParamConstraints(expr ast.Expr) []*TypeRef
```

`parseTypeRef` 补两个分支（`*ast.ParenExpr`、`*ast.UnaryExpr` 且 `Op == token.TILDE`），
让嵌套出现（如字段/返回值里的 `~T`）也能降级为对内部类型的引用。
`*ast.BinaryExpr` 只可能在类型参数约束里合法出现，交给 `parseTypeParamConstraints` 展开，
`parseTypeRef` 不特殊处理。

**2.3 `parseTypeSpec` 与 `parseFuncDecl` 改为调用 `parseGenericSpec`**

`parseTypeSpec` 中 `if spec.TypeParams != nil { ... }` 整块替换为
`t.Generic = p.parseGenericSpec(spec.TypeParams)`；
`parseFuncDecl` 中 `if decl.Type.TypeParams != nil { ... }` 整块替换为
`fn.Generic = p.parseGenericSpec(decl.Type.TypeParams)`。
行为等价且结构体/函数/方法三者约束解析完全一致。

**2.4 新增类型参数标记后处理（修缺口 3、4）**

```go
// finalizeTypeParams 在全部声明解析完成后执行：
// 对每个函数/方法，收集其签名内可见的类型参数名，然后把 Params/Results 中
// 直接以该名字为类型的 TypeRef 标记为 KindTypeParam。
func (p *Parser) finalizeTypeParams()
```

- 可见集合 = `fn.Generic.Params` 的名字 ∪（`fn.Recv != nil` 时）接收器基类型
  `pkg.Types[fn.Recv.Name].Generic.Params` 的名字。方法与其接收器类型必在同包，
  因此这一步是精确的，不依赖声明顺序。
- 遍历 `project.Packages[*].Functions` 与 `project.Packages[*].Types[*].Methods`。
- 在 `Parser.Parse` 与 `Parser.ParseProject` 末尾调用；重复执行是幂等的
  （已标记的引用再标记一次结果不变）。
- 标记规则（保守）：`TypeRef.Kind == KindStruct && PkgPath == ""` 且名字在作用域内时才改写成
  `KindTypeParam`。因此：
  - `V`（`Push[V any](v V)` 的形参）→ `{kind:"typeparam"}` ✅
  - `T`（`Pop() T` 的返回值）→ `{kind:"typeparam"}` ✅
  - `[]T` 的 `elem_type`、`map[V]W` 的 `key_type`/`elem_type` → `{kind:"typeparam"}` ✅
  - `*V` 保持 `{kind:"pointer", name:"V"}`：指针信息对调用方更有价值，`~`/指针外层不改写 ⚠️
- 结构体字段（`Type.Fields`）不在本步骤范围内。

### 3. `astp/query.go`

`ResolveTypeRef` 对 `KindTypeParam` 直接返回 `nil`，与 `KindBasic` 同样处理，
避免把类型参数误当成具名类型解析。

### 4. 测试

**4.1 fixture `astp/testdata/generic.go`**

自包含（只用同包/内置类型，不引第三方包），与现有 `testdata` 三份文件同为 `package testdata`：

```go
package testdata

type Stack[T any] struct {
    items []T
}

// @POST
func (s *Stack[T]) Push[V any](v V) {}

// @GET
func (s *Stack[T]) Pop() T { return s.items[0] }

// @GET
func (s *Stack[T]) Wrap[U ~T](u U) U { return u }

// @GET
func (s *Stack[T]) Convert[K comparable, V any](in []T) map[K]V { return nil }

// @GET
func (s *Stack[T]) Ptr[V any](v *V) {}

// @GET
func Filter[T any, R any](in []T, keep func(T) bool) []R { return nil }

// @GET
func MapKeys[K int | string](in []K) map[K]bool { return nil }
```

> 注：`astp/testdata` 被 go 工具忽略，不参与 `go build ./...` / `go test ./...` 编译，
> 因此可以安全地放入泛型方法而不需要升级 go.mod 语言版本。

**4.2 `astp/generic_test.go`**

解析 `./testdata`，断言：

- `Push`：`Generic.Params` 长度 1、`[0].Name == "V"`、`[0].Constraints[0].Name == "any"`；
  `Params[0].Type.Name == "V"` 且 `Params[0].Type.Kind == KindTypeParam`；
  `Recv.Generic.Args[0].Name == "T"` 且 `Kind == KindTypeParam`（回归：接收器实参仍正确）
- `Pop`：`Generic == nil`（仅用接收器类型参数的方法不置位）；
  `Results[0].Type.Name == "T"` 且 `Kind == KindTypeParam`（缺口 4）
- `Wrap`：`Generic.Params[0].Name == "U"`、`Constraints[0].Name == "T"`（`~T` 展开，缺口 2）
- `Convert`：两个类型参数 `K`/`V`，`Constraints[0].Name == "comparable"`；
  `in []T` 的 `elem_type` 与 `map[K]V` 的 `key_type`/`elem_type` 均被标记为 `KindTypeParam`
- `Ptr`：`*V` 刻意保持 `KindPointer`（边界行为，见下）
- `Filter`：自由泛型函数回归，`[]R` 的 `elem_type` 被标记为 `KindTypeParam`
- `MapKeys`：联合约束 `int | string` 展开为两个约束引用
- 回归：`Query.ResolveTypeRef` 对 `KindTypeParam` 返回 `nil`
- 另有一个 JSON 往返测试（Generator → Loader）确认 `KindTypeParam` 与约束不丢失

### 5. 文档

- `docs/Go1.27方法泛型支持计划.md`（本文件）
- `docs/结构设计.md`：按当前 `astp/types.go` 重写数据模型，补充方法级 `generic` 与
  `receiver.generic` 的区别、`kind: "typeparam"` 与约束的 JSON 形态
- `docs/整体解析流程.md`：改为描述 `parser.go` 的真实流水线，并标注
  「解析方法自身类型参数」与「类型参数标记后处理」两步

## 无需改动的部分（及原因）

- **`astp/types.go` 其余字段**：`Func.Recv` 与 `Func.Generic` 的职责不变，
  方法泛型只是让原本恒为 nil 的 `Func.Generic` 对方法也开始有值。
  `Func.Signatures` 目前无人写入，一并不动。
- **`astp/generator.go` / `astp/loader.go`**：纯 JSON 编解码，`KindTypeParam` 作为新枚举值
  向后兼容；无需改。
- **`astp/autoreg_generator.go`**：只关心 `@Service`/`@Controller`/`@Middleware` 注解与
  零参构造函数，与类型参数无关；无需改。
- **`astp/cmd/astp`**：CLI 无需新参数。
- **`go.mod`**：见上「是否升级 go.mod」。`go/parser` 的解析行为由**运行时的工具链**决定，
  与 go.mod 声明的语言版本无关（本次改动在 `go 1.25` 声明下实测通过）。
- **`fw/app`、`fw/openapi` 等消费方**：本步骤仅到 astp 包修改完成，消费方适配不在范围内。

## 验证

1. `go build ./astp/...`
2. `go test ./astp/...`（新增泛型方法测试 + 既有测试）
3. `gofmt -l astp`
4. 重新生成 `astp/testdata/.astp.json` 使已提交的产物与解析结果一致

> 注意：`go test ./...` 在当前工作区存在**与本次改动无关的既存编译失败**
> （`app/engine.go` 与 `cmd/example` 仍在用旧的 `config.Load` 两返回值签名和
> `config.Section`/`Config.Server`/`Config.OpenAPI` 等字段，与新配置库不匹配）。
> 该失败在本次改动之前即可复现，不在本步骤修复范围内。

## 边界与说明

- **同名遮蔽**：方法自身类型参数与接收器实参同名（Go 允许遮蔽，如
  `func (s *Stack[T]) M[T any](t T)`）时，作用域集合是一个 set，两个同名条目合并为一个，
  标记行为一致，属可接受行为。
- **`~` 不落库**：`Wrap[U ~T]` 的 `U` 约束记录为 `[{name:"T", kind:"struct"}]`，
  与 `[U T]` 同形。需要区分时应在 `GenericParam` 上新增字段，属后续增强。
- **`interface{...}` 约束**：`M1[V interface{ ~int | string }]` 展开为
  `[{name:"int"},{name:"string"}]` 两个约束引用，接口外层信息丢弃
  （`interface{}`/`any` 仍是 `{name:"any"}`）。
- **约束引用的 `kind`**：由 `isBasicType` 决定——`int`/`string` 等真正内置类型是 `"basic"`，
  而 `any` / `comparable` 不在该名单里，因此是 `"struct"`（含义只是「非内置的裸名字」）。
  这是 `parseTypeRef` 的既有行为，本次未改动。
- **字段侧不标记**：`Type.Fields` 里的类型参数引用仍是 `kind: "struct"`，
  本步骤只覆盖 `Func.Params` / `Func.Results` / `Func.Recv`。
- **指针外层不改写**：`*V` 保持 `{kind:"pointer", name:"V"}`，因为指针信息对参数绑定更有价值；
  代价是 `ResolveTypeRef` 仍会尝试按名字查找。若出现同名具名类型会误解析，
  这一情形罕见，文档说明即可。
- **跨包类型参数**：`demo.TestStruct` 这类带 `pkg_path` 的引用不会被标记
  （标记条件要求 `PkgPath == ""`），符合预期。
- **静态占位**：方法自身类型参数的具体类型只在调用点可知，astp 作为静态解析器
  只保留「已声明」的占位，不做实例化替换，也不推断调用点的实参。
