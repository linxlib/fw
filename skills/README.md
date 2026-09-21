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
| `fw-development` | 修改 fw 框架源码、在 fw 应用项目中开发、或需要提交代码/更新文档时的强制规范 |

## 与仓库文档的关系

skill 是**执行清单**，不是文档副本。权威详版文档仍然是仓库根目录的：

- `AGENTS.md` — 框架架构与约定
- `APP_AGENT.md` — 应用开发指南
- `DOCS_AGENT.md` — 文档维护规则

skill 只负责回答"现在该读哪一份、以及哪些事不做就算没做完"。规则变更时先改上述文档，再同步 skill。
