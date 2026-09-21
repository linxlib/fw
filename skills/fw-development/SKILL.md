---
name: fw-development
description: 在 github.com/linxlib/fw 仓库本身或基于 fw 的 Go 应用项目中开发时必须遵守的规范与工作流。涵盖框架开发与应用开发的边界、提交信息风格、文档三件套同步义务、交付前检查清单。当任务涉及修改 fw 框架源码、在 fw 应用项目里加功能、或需要提交代码/更新文档时使用本 skill。
---

# FW 开发规范

本 skill 是 fw 生态开发的**强制执行清单**。仓库里的 `AGENTS.md`、`APP_AGENT.md`、`DOCS_AGENT.md` 是权威详版文档，本 skill 负责告诉你**什么时候该读哪一份、以及哪些事不做就算没做完**。

## 第 0 步：先判断你在改什么

这一步决定后面所有规则，不要跳过。

| 你在哪里 | 权威文档 | 边界 |
|---|---|---|
| fw 框架仓库（含 `app/` `astp/` `config/` `inject/` 等） | `AGENTS.md` + `DOCS_AGENT.md` | 可以改框架 |
| 业务应用项目（`import "github.com/linxlib/fw/v2"`） | `APP_AGENT.md` | **把 fw 当外部依赖，不要改框架** |

如果是应用项目，只有用户明确要求做框架开发时才动框架源码；否则再丑的框架行为也应在应用层绕过。改完框架后，记得同步上面表格里的文档。

## 提交信息规范

严格使用 Conventional Commits，**描述用中文**，scope 用小写包名：

```
<type>(<scope>): <中文描述>
```

- `type`：`feat` / `fix` / `docs` / `refactor` / `test` / `chore`
- `scope`：出问题的那个包，如 `config`、`openapi`、`middleware`、`astp`、`fw`、`app`；跨包或无明确归属时可省略
- 正文用 `- ` 列条目，每条一行，说清"改了什么行为"而不是"改了哪个文件"

正确示例（来自仓库真实历史）：

```
feat(config): 检测到需要重载时在标准输出提示变化的 section
fix(middleware): 同一中间件跨层绑定时只执行一次
fix(fw): 修正集合路由尾斜杠、缺省查询参数 500 与同类型形参串值
docs: 同步 v2.0.1 以来的变更到全部 markdown 文档
```

反例：`update code`、`fix bug`、`修改了一些文件`——没有 type、没有行为说明。

## 交付前必须全部通过

```powershell
gofmt -l .          # 必须无输出（注意：仓库里 config/env.go 历史遗留未格式化，不要顺手改）
go vet ./...        # 必须无输出
go test ./...       # 必须全绿
```

三条全过再提交。任一条失败就停下修，不要提交后补。

## 框架开发的额外约束

- **行为必须确定性**：注解合并、中间件顺序、路由匹配这类核心行为不允许出现"视情况而定"
- **`astp` 与 `inject` 保持仓库内子包**，在仓库内演进，不拆出去
- **优先向后兼容**：破坏性变更（改签名、删导出 API）要有明确理由，且必须在提交信息正文里说明影响
- **不留未文档化的用户可见行为**：见下一节
- 新增导出 API 时同步 `AGENTS.md` 的对应小节

## 文档同步义务（最容易漏的一项）

规则来自 `DOCS_AGENT.md`，核心一句话：**用户可见的改动，三个文档必须同一个任务内改完**。

必须同步的文件：

| 文件 | 定位 |
|---|---|
| `doc.md` | 短入口页，只放概览/快速开始/CLI 摘要 |
| `doc_en.md` | 英文完整指南 |
| `doc_cn.md` | 中文完整指南，必须与 `doc_en.md` **语义对齐、章节对齐**，不是 loosely related |

触发条件（命中任意一条就要改文档）：

- 框架特性增减
- 公开 API 变化
- 注解语义变化
- 中间件行为、参数绑定行为、DI 行为、响应格式行为、OpenAPI 生成行为
- 配置结构、默认值、环境变量名
- `cmd/fw` 的命令、flag、生成物
- 文档里的示例代码本身过期了

同步时的硬性要求：

1. 三个文件同一次改完，不要留作 follow-up
2. `doc_en.md` 与 `doc_cn.md` 章节标题与顺序保持一致
3. 示例必须匹配当前代码真实行为，不能写想象中的 API
4. 已删除的特性要**删掉对应章节**，而不是留一段矛盾的说明
5. 改完通读三个文件，确认互相不矛盾

自查命令：

```powershell
Select-String -Path doc.md,doc_en.md,doc_cn.md -Pattern '过期的关键词'
# 确认代码围栏成对
(Get-Content doc_cn.md | Where-Object { $_ -match '^\s*```' }).Count   # 必须是偶数
```

## 应用开发速查

分层决策（有疑问时按这个来）：

- `controllers/` — HTTP 入口与路由注解
- `services/` — 业务规则，标 `@Service`
- `middlewares/` — 横切行为，**不要放业务逻辑**
- `models/` — 请求/响应结构
- `config/app.yaml` — 环境相关行为

注解要点：

- 路由：`@GET` `@POST` `@PUT` `@DELETE` `@PATCH` `@OPTIONS` `@HEAD`，控制器基路径用 `@Route /base`
- 同一方法可写多条同方法注解，只要 URL 不同
- 模型命名帮助推断绑定来源：`XxxBody` / `XxxQuery` / `XxxPath` / `XxxHeader`，必要时用 `@Body` `@Query` 显式指定
- 模型字段加 `example:"..."` 与 `default:"..."`，OpenAPI 才有真实示例值
- 泛型基类：`type UserController struct { BaseController[models.User] }`，方法提升后 schema 按实参展开

**加了带注解的类型后必须重新生成元数据**，否则 `go run .` 用的是旧 `.astp.json`：

```powershell
fw build      # 或至少 go generate ./...
go test ./...
```

需要配置热重载时，在 `app.New` 之后、`ListenAndServe` 之前调用 `e.EnableConfigReload(time.Second)`。注意它只更新内存配置，**不会重建路由表、不会切换监听端口**。

## 常见陷阱

- 改完框架忘改文档 → 交付不完整
- 提交信息写英文流水账 → 不符合仓库风格
- 把业务逻辑塞进中间件或控制器
- 只改 `doc_en.md` 不改 `doc_cn.md`
- 在应用项目里顺手改框架
- 假设路径参数一定能绑上：路由声明了但处理方法没写形参（或反过来）会直接报错，不会静默给空值
