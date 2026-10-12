# Service Actor Guards

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

Make the three service-layer actor guards sudo-aware, per D3 in the [handler inventory](../notes/handler-inventory.md#planning-decisions), so a sudo-only identity that passes the sudo-aware `RequireAuth` gets 403, not a sudo-unaware 401. Touches only `api/service`.

## Requirements

1. In `TagService.Create` (`tag.go:138`), `TagService.Search` (`:313`) and `TagService.ListBySubject` (`:417`):
   - first `authz.RequireAuthenticated(ctx)` (return its error);
   - then `opctx.ActorEntityID(ctx)`; if absent (sudo-only), return `ErrForbidden`;
   - keep the regular actor id as owner/scope exactly as today (no `EffectiveActorEntityID`).
2. Update the doc comments ("Returns ErrUnauthenticated if no actor is present in ctx", and the inline comments citing `ErrUnauthenticated`).
3. Tests (prefix `tgSudo`) in `api/service`: no identity gives `ErrUnauthenticated`; sudo-only gives `ErrForbidden` with no write and no `Authorize` call; regular actor unchanged.
4. Do not touch `api/httpapi` (task 7.3 may be running) and do not reorder validation here (task 8.1 does).
5. Append the in-process entry to the [wire-risk note](../notes/wire-risk.md#recorded-by-tasks).

## Validation

From `api/` ([build setup](../notes/handler-inventory.md#building-and-validating-from-a-task-worktree)):

```bash
test -z "$(gofmt -l .)" || gofmt -l .
go vet ./...
go vet -tags=integration ./...
go test ./...
git diff --stat -- . ':!plan/'   # api/service only
```

## Metadata

architectural_impact: false

## Assumptions

- Task 7.2 has landed. Task 7.3 may run at the same time.

## Status

- Outcome: succeeded (2026-10-11).
- Validation: gofmt clean, `go vet ./...`, `go vet -tags=integration ./...`, `go test ./...` all pass; diff limited to `api/service` (plus plan notes).
- Files: `api/service/tag.go` (new `requireRegularActor` helper used by Create/Search/ListBySubject), `api/service/errors.go` (doc comment), `api/service/tag_sudo_test.go` (`tgSudo` tests).
