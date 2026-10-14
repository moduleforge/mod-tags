# State-Conflict Audit

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

Classify every post-authorization error left in mod-tags (audit axis 5 of [common section 2](/Users/zane/playground/moduleforge/mod-core/plan/waves/http-error-precedence-conformance/common-conformance-instructions.md#2-per-handler-conformance-audit-procedure)), with the wave manifest's explicit question first: is the `one_of_domain` rejection a 409 or a 422? The planned answer and its evidence are D8 in the [handler inventory](../notes/handler-inventory.md#planning-decisions); this task decides with a recorded rationale and pins the result with tests.

## Requirements

1. Enumerate every `ErrInvalidInput`/`ErrNotFound`/`ErrConflict` return reachable after an `Authorize` call in `api/service/tag.go` (and `tag_template.go` if any route reaches one). Record each with its decision and a one-line rationale in a "State conflicts" list under the [audit table](../notes/handler-inventory.md#audit-table).
2. `one_of_domain`: planned to stay 409 `conflict` (uniqueness against an existing tag; the GUI's `TagEditor` depends on the 409). If you conclude 422 is right, do NOT change it: record the reasoning and flag it in the report as a decision for the manager, since it would break `TagEditor`.
3. Pin the mapping with tests (prefix `tgConf`):
   - a `CreateTag` failure with SQLSTATE 23505 (as the trigger raises it, without a constraint name) yields `ErrConflict` after a successful authorize;
   - a denied caller gets 403 and the insert is never attempted (the conflict cannot leak to an unauthorized caller);
   - existing one-of-domain integration tests still assert 409/`ErrConflict` (run them if Postgres is reachable; see the [build setup](../notes/handler-inventory.md#building-and-validating-from-a-task-worktree)).
4. Confirm or flag D4's post-authorization `GetByUUID` 404 (`tag.go:284`) and `Create`'s internal owner-entity 404 (`:191`). Do not re-status a capability gap; flag it (common section 2 axis 5).
5. If the audit changes nothing on the wire, say so in the [wire-risk note](../notes/wire-risk.md#recorded-by-tasks).

## Validation

From `api/` ([build setup](../notes/handler-inventory.md#building-and-validating-from-a-task-worktree)):

```bash
test -z "$(gofmt -l .)" || gofmt -l .
go vet ./...
go vet -tags=integration ./...
go test ./...
```

The "State conflicts" list exists and every entry has a decision and rationale.

## Metadata

architectural_impact: false

## Assumptions

- Phase 08 has landed.

## Status

- Outcome: succeeded (2026-10-14).
- Decision: `one_of_domain` stays 409 (D8 confirmed); "State conflicts" list added to `plan/notes/handler-inventory.md` with a decision and rationale per entry, including a precise description of `POST /tags` subject authorization (finding `p3Gt`, not changed).
- Code: stateless NUL validator for the `ListBySubject` purpose filter in `api/service/tag.go`; tests in `api/service/tag_conflict_test.go` (`tgConf` prefix).
- Validation: gofmt clean, `go vet ./...`, `go vet -tags=integration ./...`, `go test ./...` pass. Integration suites were skipped (Docker unreachable: "docker inspect: exit status 1"), so the existing one-of-domain integration tests were not run; the mock-level one-of-domain conflict tests still assert `ErrConflict`.
- Wire-risk note updated.
