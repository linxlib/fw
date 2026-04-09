# Documentation Update Guide For Coding Agents

This document tells coding agents how to maintain the framework documentation after changing framework behavior.

Target docs:

- `doc.md`
- `doc_en.md`
- `doc_cn.md`

## Goal

When you add, remove, rename, or change a framework feature, keep the public documentation in sync in the same task whenever possible.

Do not leave user-facing behavior changed while docs still describe the old behavior.

## When You Must Update The Docs

Update the three documentation files whenever your code change affects any of the following:

- framework features
- public APIs
- annotations
- middleware behavior
- request parameter binding behavior
- dependency injection behavior
- response format behavior
- OpenAPI generation behavior
- configuration structure, defaults, or environment overrides
- CLI commands, flags, output, or workflow
- project scaffolding templates
- recommended development flow
- examples that are shown in the docs

Typical cases that require doc updates:

- adding a new annotation such as `@XXX`
- adding a new middleware capability or middleware merge rule
- changing route registration behavior
- changing request binding rules for query, path, header, or body
- changing `cmd/fw` command names, flags, examples, or defaults
- changing generated project structure from `fw init`
- changing response envelope fields
- changing Swagger or OpenAPI routes or schema behavior
- changing configuration keys or environment variable names
- introducing a new recommended pattern such as SSE or WebSocket support
- removing or deprecating an old capability

## When You Usually Do Not Need To Update The Docs

Doc updates are usually not required for:

- purely internal refactors
- test-only changes
- performance-only changes with no user-visible behavior change
- log wording changes that do not affect usage
- private helper renames

If unsure, prefer updating docs.

## Responsibilities Of Each Doc File

### `doc.md`

This is the short entry document.

Keep it:

- short
- easy to scan
- useful as the default landing page
- linked to `doc_en.md` and `doc_cn.md`

Update `doc.md` when:

- the framework feature list changes
- the quick start changes
- the recommended CLI examples change
- the relationship between the two full guides changes

### `doc_en.md`

This is the full English guide.

It should explain:

- what the feature is
- when to use it
- how to use it
- examples
- important rules and edge cases

### `doc_cn.md`

This is the full Chinese guide.

It should stay semantically aligned with `doc_en.md`, not just loosely related.

It does not need to be a word-for-word translation, but it must cover the same user-visible behavior and constraints.

## Update Rules

When behavior changes, follow these rules:

1. Update all three docs in the same task.
2. Keep `doc_en.md` and `doc_cn.md` aligned in scope.
3. Keep `doc.md` as a concise overview, not a duplicate of the full guides.
4. Prefer examples that match the current codebase behavior.
5. Do not document features that are not implemented.
6. Do not keep stale examples after renaming APIs, annotations, or commands.
7. If a feature is removed, delete or rewrite the related doc sections instead of leaving contradictory notes.

## Recommended Update Workflow

After implementing a framework change:

1. Identify whether the change is user-visible.
2. Find every affected section in `doc.md`, `doc_en.md`, and `doc_cn.md`.
3. Update the short overview in `doc.md` if needed.
4. Update the full explanation and examples in `doc_en.md`.
5. Mirror the same meaning in `doc_cn.md`.
6. Re-read the changed sections to make sure the three docs do not contradict each other.

## Section Checklist

When changing a feature, check whether these sections need updates:

- feature list
- quick start
- startup flow
- annotation usage
- controller usage
- service usage
- middleware usage
- dependency injection
- parameter binding
- context API
- response model
- OpenAPI and Swagger
- configuration
- recovery and logging
- manual registration
- `cmd/fw` CLI
- recommended development flow
- example code blocks

## Special Rules For CLI Changes

If you change `cmd/fw`, check all of these:

- command names
- subcommand names
- flags
- defaults
- examples
- generated project layout
- generated file contents if they affect documented usage

Preferred verification:

- read the CLI source in `cmd/fw/`
- when practical, verify help output with `go run ./cmd/fw --help` and related subcommands

## Special Rules For Annotation Changes

If you add or change annotations:

- document the supported syntax
- document where the annotation can be used
- document merge or override behavior if relevant
- document any interaction with middleware, routing, binding, or OpenAPI
- update examples

## Special Rules For Config Changes

If you change config behavior:

- update YAML examples
- update default values if they changed
- update environment variable examples if they changed
- update middleware config examples if affected

## Special Rules For Binding And Response Changes

If you change request binding or response behavior:

- update the rule description
- update any example handler signatures
- update example request models
- update response JSON examples
- update any OpenAPI-related explanation if schema generation is affected

## Writing Style

For all three docs:

- keep explanations simple and direct
- explain behavior from a framework user perspective
- prefer short code examples
- use examples that match actual framework behavior
- avoid promising future or unimplemented features

For `doc_en.md` and `doc_cn.md`:

- keep headings roughly aligned so future maintenance is easier
- keep terminology consistent

## Final Self-Check Before Finishing

Before finishing a task that changes user-visible framework behavior, confirm:

- `doc.md` still works as the short landing page
- `doc_en.md` reflects the current implementation
- `doc_cn.md` reflects the same behavior as `doc_en.md`
- examples still match the real API shape
- removed features are no longer documented
- new features are documented in the right sections

## Minimum Expectation

If a framework change is user-visible, the minimum acceptable outcome is:

- update `doc_en.md`
- update `doc_cn.md`
- update `doc.md` if the change affects overview, quick start, or CLI summary

Do not treat documentation sync as optional follow-up work.
