---
created: 2026.06.21
creators:
  - project-flow-check skill
notes: Session-binding manifest produced by `project-flow-check`. Regenerated on each run.
---

# Flow Skill Binding

## Project type

- language: multi-language (Go for api/model; TypeScript for gui)
- framework: React (via @ladle/react component workbench)
- runtime: Node / npm (gui/ sub-project — target: bun)
- additional markers: Makefile present; Go sub-projects (api/, model/)

## Build / test / run commands

| purpose | command | source |
|---------|---------|--------|
| build | `make build` | Makefile |
| test | `make test` | Makefile |
| clean | `make clean` | Makefile |
| dev/preview | `make preview` | Makefile |
| gui build | `cd gui && npm run build` | Makefile → gui/package.json |
| gui typecheck | `cd gui && npm run typecheck` | Makefile → gui/package.json |
| gui dev | `cd gui && npm run dev` | gui/package.json scripts |

## Layout conformance

| dimension | score | notes |
|-----------|-------|-------|
| standard doc set | absent | No README.md at root; no AGENTS.md |
| docs/ discoverability | n/a | No docs/ directory |
| plan/ shape | present | overview.md, TODO.yaml present in plan worktree |
| make-layout | partial | build/test/clean present; no run/start target |

## Bound skill chain

- role docs: `references/role/developer-node.md`, `references/role/developer-go.md`
- doc-author skills: `write-readme`, `write-agents-md`, `write-project-spec`
- implementation skills: `implement-task` (via `dispatch-implementation-task`)
- review skills: `review-changes-correctness`, `review-changes-style`, `review-changes-security`, `review-changes-efficiency`
- release skills: `package-release`, `coordinate-release`
- deploy / sunset / archive skills: none detected

## Link-chain status

- root: none (README.md absent)
- first-layer docs: none
- depth: 0
- orphans: next-steps.md, stories-next.md (unreachable — no root)

## Open binding gaps

- README.md missing at project root — link-chain cannot be established.
- AGENTS.md missing — no documented build/test commands at source level.
- gui/ uses npm; bun migration plan in progress (see Active plans).

## Active plans

| Slug | Branch | Worktree | Status |
|------|--------|----------|--------|
| bun-migration | plan/bun-migration | /Users/zane/playground/moduleforge/mod-tags/worktree/plan/bun-migration | healthy |
