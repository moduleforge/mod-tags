# Full Validation

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

The mod-tags plan-wide validation pass: full api and model suites, lint, the integration suites, checklist greps, and a final read-through confirming every audit-table row is closed ("Validation" item of [common section 6](/Users/zane/playground/moduleforge/mod-core/plan/waves/http-error-precedence-conformance/common-conformance-instructions.md#6-standard-per-plan-task-template)). Fix nothing larger than a typo; file anything else as a finding (`followups_add`, `plan_slug: tasks-tags-conformance`).

## Requirements

1. From the worktree root ([build setup](../notes/handler-inventory.md#building-and-validating-from-a-task-worktree)): `make -C api lint`, `make -C api test`, `cd model && go vet ./... && go test ./...`, and `cd api && go vet -tags=integration ./...`. If `gui/node_modules` exists, also run the root `make test`; otherwise report it as not run. Do not run `bun install`.
2. Run the integration suites (`go test -tags=integration -p 1 -count=1 -v ./service/...` from `api/`). Report "passed" only if they ran; a skip is `partial` for this requirement.
3. Checklist greps (non-test code in `api/`), zero hits unless stated:
   - `grep -rn 'json.NewDecoder(r.Body)\|uuid.Parse(' httpapi/`
   - `grep -rn 'ActorEntityID\|ErrUnauthenticated' httpapi/`
   - `grep -n 'authz.RequireAuth' httpapi/router.go` gives exactly 1, inside `r.Group`
   - `grep -cE 'r\.(Get|Post|Put|Patch|Delete)\(' httpapi/router.go` gives 8
   - `grep -c 'apiresp.DecodeJSON' httpapi/tags.go` gives 3
   - every `opctx.ActorEntityID` in `service/*.go` is preceded in its method by `authz.RequireAuthenticated` (manual read)
   - in `TagService.Create`/`.Search`, `Authorize` follows every validation return (manual read)
4. Read the [audit table](../notes/handler-inventory.md#audit-table) and mark each of the 8 rows closed, deferred (with a finding id), or open (report it).
5. Confirm `go.mod`, `go.sum`, `bun.lock` and `package.json` are unchanged against the plan base: `git diff 4e58869 -- '*.mod' '*.sum' 'bun.lock' '*package.json'` is empty. No push has been made.

## Validation

- All commands in requirements 1-2 exit 0, or the report explains each non-zero exit.
- The checklist results and the closed audit table are in the report.

## Metadata

architectural_impact: false

## Assumptions

- Task 10.1 has landed.

## Status

Outcome: succeeded, partial on requirement 2 (2026-10-14).

- `make -C api lint`, `make -C api test`, `model` vet+test, `api` `go vet -tags=integration`: exit 0.
- Root `make test`: exit 2, environmental only (gui `@moduleforge/core-gui` unresolved in the worktree); Go side passed.
- Integration suites: SKIPPED. `go test -tags=integration` exited 0 but the TestMain skipped the tag grant suites: `docker inspect: exit status 1` (container `users-module-postgres` does not exist; fixed port 5432 is held by an unrelated host postgres).
- Checklist greps: all as expected (no decoder/uuid.Parse/ActorEntityID/ErrUnauthenticated hits in httpapi non-test code; one `r.Use(authz.RequireAuth)` inside `r.Group`; 8 routes; 3 `DecodeJSON`; the only service `opctx.ActorEntityID` is in `requireRegularActor` after `authz.RequireAuthenticated`; `Create`/`Search` call `Authorize` after all validation returns).
- Audit rows 1-8: closed, each covered by a passing `TestOrder_*` test in `api/httpapi` or `api/service/tag_routes_order_test.go`.
- `git diff 4e58869` over mod/sum/bun.lock/package.json: empty.
