# FW Skills

本目录存放 fw 生态的开发规范 skill，供 coding agent 加载。

## 目录约定

每个 skill 一个子目录，入口文件为 `SKILL.md`，格式为 YAML frontmatter + Markdown 正文：

```markdown
---
name: <skill 名>
description: <什么时候该加载本 skill，写清楚触发场景>
---

<指令正文>
```

`description` 是 agent 判断是否加载的唯一依据，必须写清**适用场景**，不要写成功能简介。

## 当前 skill

| 名称 | 用途 |
|---|---|
| `fw-development` | 修改 fw 框架源码本身时的强制规范：提交风格、交付检查、框架约束、文档三件套同步 |
| `fw-app-development` | 在基于 fw 的业务应用项目中开发功能：新增 API 流程、分层职责、注解与绑定、元数据重新生成 |

两个 skill 的分界：动框架源码用前者，写业务代码用后者。两者内容不重叠，规则变更时同步改，避免出现矛盾要求。

## 与仓库文档的关系

skill 是**执行清单**，不是文档副本。权威详版文档仍然是仓库根目录的：

- `AGENTS.md` — 框架架构与约定
- `APP_AGENT.md` — 应用开发指南
- `DOCS_AGENT.md` — 文档维护规则

skill 只负责回答"现在该读哪一份、以及哪些事不做就算没做完"。规则变更时先改上述文档，再同步 skill。

## 文档不在手边时怎么办

两个 skill 都只是**执行清单**，完整规范在 fw 框架仓库的 markdown 里，**不在应用项目内**：

| 文件 | 内容 |
|---|---|
| `APP_AGENT.md` | 应用开发完整指南（`fw-app-development` 的权威来源） |
| `doc_cn.md` / `doc_en.md` | 中/英文完整框架文档 |
| `doc.md` | 框架概览 |
| `AGENTS.md` | 框架架构与模块地图 |
| `DOCS_AGENT.md` | 文档维护规则（`fw-development` 的权威来源之一） |
| `config/README.md` | 配置库完整说明 |

仓库地址 `https://github.com/linxlib/fw`，分支 `v2`，上述文件都在仓库根目录。

**文档和代码同一个 tag**，先看 `go.mod` 里 `github.com/linxlib/fw/v2` 的版本号，再读同版本文档，避免文档描述与实际依赖不一致。

怎么读不规定命令：agent 的 shell 可能是 bash / zsh / git bash / WSL / cmd / PowerShell，用顺手的方式即可。已有源码检出就直接读；否则只取需要的那几个文件，读完即弃。**不要为文档去 clone 整个仓库，也不要去翻 Go 模块缓存目录**（在工作目录以外，且版本不一定匹配）。详见 `fw-app-development/SKILL.md` 的「第 1 步：确认权威详版文档在哪」。
