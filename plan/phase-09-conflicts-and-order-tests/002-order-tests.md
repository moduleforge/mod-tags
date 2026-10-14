# Order Tests

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

Build the mod-tags order-test harness and cover all 8 routes, following the [order-test recipe](/Users/zane/playground/moduleforge/mod-core/plan/waves/http-error-precedence-conformance/common-conformance-instructions.md#5-order-test-recipe-apirespordertest) and D9 (approach B) in the [handler inventory](../notes/handler-inventory.md#planning-decisions). Working examples: `/Users/zane/playground/moduleforge/mod-audit/api/httpapi/order_test.go` and `/Users/zane/playground/moduleforge/mod-core/api/httpapi/order_helpers_test.go`. Test-only.

## Requirements

1. `api/service/export_test.go` (package `service`): a constructor that builds `*Services` like `New` but with injected `newCoreQuerier`/`newTagQuerier` factories, and aliases or constructors for the fakes the tests need (`fakeDB`, `mockCoreQuerier`, `mockTagQuerier`, `stubOpResolver`, `recordingObserver`). Exported test symbols start with `Order`.
2. `api/service/order_helpers_test.go` (package `service_test`, prefix `order`): an authorizer that allows the owner on its own entity and on the seeded tag, denies the stranger, and denies target 0 quietly (common rule 5); seeds (owner, stranger, subject entity, one tag, a one-of-domain purpose policy if the fakes model it, a tag template) and well-formed missing UUIDs; `orderRouter()` building the real `httpapi.NewRouter(httpapi.NewDeps(...))` with no identity middleware. If approach B cannot work, use D9's fallback and record why in the [audit table](../notes/handler-inventory.md#audit-table).
3. `api/service/tag_routes_order_test.go` (package `service_test`, prefix `tgOrd`), one `ordertest.Route` per route:
   - UUID routes (`GET`/`PUT`/`PATCH`/`DELETE /tags/{uuid}`, `GET /entities/{uuid}/tags`): `AuthorizedMissingStatus: 403` (D4);
   - `BadBody`: a decode-layer rejection, plus extra `Route`s for the handler and service stateless rules (PUT without `color`, PATCH without `value`, POST with a blank purpose or a bad color);
   - `Late`: the one-of-domain 409 on `POST /tags` if the fakes can express it (otherwise covered by task 9.1's service test; say so);
   - `POST /tags` and `GET /tags` (no path id): plain table tests for 401, 400 (including "neither owner nor subject") and 403, plus sudo-only 403 (D3);
   - `GET /tag-templates` (authn-only, no 403 stage): a dedicated test asserting anonymous plus missing `purpose` is 401, an authenticated caller with missing `purpose` or a bad `scope` is 400, a sudo-only identity is 200, and no `Authorize` call is ever made.
4. Show the suite bites (recipe step 6): temporarily move `Create`'s validation back after `Authorize`, observe the failure, revert, and report it.
5. No production code changes. If a route cannot pass without one, halt and report.
6. Record which test covers each of the 8 rows in the [audit table](../notes/handler-inventory.md#audit-table).

## Validation

From `api/` ([build setup](../notes/handler-inventory.md#building-and-validating-from-a-task-worktree)):

```bash
test -z "$(gofmt -l .)" || gofmt -l .
go vet ./...
go test ./service/ -run 'Order' -v 2>&1 | grep -E -- '--- (FAIL|SKIP)'   # no output
go test ./...
git diff --stat -- . ':!plan/'   # only api/service/*_test.go files
```

## Metadata

architectural_impact: false

## Assumptions

- Task 9.1 has landed.

## Checkpoint hints

- After the harness compiles and one route passes.
- After all 8 routes pass.

## Status

- Outcome: succeeded (2026-10-14).
- Validation: gofmt clean; `go vet ./...` and `go vet -tags=integration ./...` clean; `go test ./service/ -run Order -v` shows no FAIL or SKIP (166 passing subtests); `go test ./...` passes. Integration suites were not run (no Postgres/Docker claimed). Diff is test-only: `api/service/export_test.go`, `api/service/order_helpers_test.go`, `api/service/tag_routes_order_test.go`, plus the audit-table coverage section in `plan/notes/handler-inventory.md`.
- Bite check: moving `Create`'s validation after `Authorize` failed the `POST /tags` 400 cases; reverted.
- Notes: approach B worked, no fallback. The `one_of_domain` 409 is covered through ordertest `Late` (fakes model the trigger). Finding `p3Gt` is pinned by `TestOrder_PostTags_SubjectNotAuthorized_p3Gt`. POST/GET collection routes use plain table tests per the task.
