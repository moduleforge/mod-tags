# Audit Handlers

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

A read-only audit of all 8 mod-tags routes (`api/httpapi/router.go`) and every service method they reach in `api/service/{tag,tag_template}.go`, following the per-handler procedure in [common section 2](/Users/zane/playground/moduleforge/mod-core/plan/waves/http-error-precedence-conformance/common-conformance-instructions.md#2-per-handler-conformance-audit-procedure). The output is the authoritative audit table later tasks work from. No source changes.

## Requirements

1. Verify every row of the [preliminary route table](../notes/handler-inventory.md#preliminary-route-table) and every planning decision D1-D11 against source, reading each handler AND the service method beneath it.
2. Append the five-axis table under [Audit table](../notes/handler-inventory.md#audit-table), one row per route (8 rows), with: route class and authn gate; every stateless check and whether it runs before or after the first resolve/authorize; the authorize call and target; the resolve/existence classification; post-authorization errors with the planned classification; the wire-visible and in-process effects.
3. Specifically settle:
   - D4: that no route returns anything distinguishable for a missing versus a forbidden tag or subject before the authorize decision (including `Search`'s owner/subject filter resolution and `TagTemplateService.List`'s scope resolution, which has no authorization by design);
   - D3: `grep` non-test code for `opctx.ActorEntityID`; list every read;
   - D8: whether any DB constraint or trigger other than `tags_enforce_one_of_domain` can raise SQLSTATE 23505 on `CreateTag` (read `model/migrations/`), and whether any CHECK constraint can surface a post-authorization 500 for input the service validation accepts (the byte-length vs. `char_length` mismatch between `len(in.Purpose) > 512` in Go and `char_length(purpose) <= 512` in SQL is already filed as finding `BTWb`; do not refile it);
   - whether `GetByUUID`'s post-authorization `ErrNotFound` (`tag.go:284`) is reachable at all.
4. Where source contradicts the note, correct the preliminary table in place and list each correction in your report. Do not change a decision yourself; flag it.
5. File genuinely out-of-scope findings with `followups_add` (`plan_slug: tasks-tags-conformance`), per common rule 9.

## Validation

- `git diff --stat -- . ':!plan/'` is empty (no source changes).
- `plan/notes/handler-inventory.md` has 8 audit rows under `## Audit table`.
- `grep -cE 'r\.(Get|Post|Put|Patch|Delete)\(' api/httpapi/router.go` is 16 today (8 in `NewRouter`, 8 in `RegisterRoutes`); the audit covers the 8 distinct routes.

## Metadata

architectural_impact: false

## Assumptions

- Runs in parallel with task 7.2 (test fakes only). `go build ./...` passes; the test build is red until 7.2 lands.

## References

- [Common conformance instructions](/Users/zane/playground/moduleforge/mod-core/plan/waves/http-error-precedence-conformance/common-conformance-instructions.md): sections 1, 2 and 4.
- `AGENTS.md` "Router mounting" and "Conventions"; `docs/decisions/tags-one-of-domain.md`.
- [Handler inventory](../notes/handler-inventory.md) and [wire-risk note](../notes/wire-risk.md).
