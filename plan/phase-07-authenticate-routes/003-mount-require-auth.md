# Mount Require Auth

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

Make 401 the first rejection on every mod-tags route by mounting `authz.RequireAuth` inside `RegisterRoutes` (`api/httpapi/router.go:50`), and make `NewRouter` (`:28`) delegate to `RegisterRoutes` so both entry points share the gate. Placement follows D1 in the [handler inventory](../notes/handler-inventory.md#planning-decisions) and the `RequireAuth` row of [common section 3](/Users/zane/playground/moduleforge/mod-core/plan/waves/http-error-precedence-conformance/common-conformance-instructions.md#3-helper-reference-final-signatures). The inline checks stay for now (task 7.5, common rule 1).

## Requirements

1. `NewRouter`: replace its 8 duplicated registrations with `r := chi.NewRouter(); RegisterRoutes(r, d); return r`. Keep its signature and doc comment (updated).
2. `RegisterRoutes`: register all 8 routes inside `r.Group(func(r chi.Router) { r.Use(authz.RequireAuth); ... })`. Never call `Use` on the router passed in. `GET /tag-templates` is inside the group (authenticated) but gets no authorization call (D2).
3. Tests (prefix `tgAuthn`), table-driven over all 8 routes, through `NewRouter` and once through `RegisterRoutes` on a bare `chi.NewRouter()`:
   - anonymous with a well-formed request gets 401;
   - anonymous with a malformed path UUID, malformed body, or invalid query (`GET /tag-templates` without `purpose`, `GET /tags?owner=bad`) gets 401, not 400;
   - a dummy route registered next to the group on the same bare router is not gated.

   Prove the mount is what makes them pass: temporarily delete one handler's inline check and run the new test without the mount (red) and with it (green). Report it; do not commit the mutation.
4. Update the package docs (`router.go`, `doc.go`) to say `RegisterRoutes` enforces authentication with `authz.RequireAuth`.
5. Append the entry to the [wire-risk note](../notes/wire-risk.md#recorded-by-tasks).

## Validation

From `api/` ([build setup](../notes/handler-inventory.md#building-and-validating-from-a-task-worktree)):

```bash
test -z "$(gofmt -l .)" || gofmt -l .
go vet ./...
go test ./...
grep -cE 'r\.(Get|Post|Put|Patch|Delete)\(' httpapi/router.go   # 8 (no duplicated registrations)
grep -n 'authz.RequireAuth' httpapi/*.go | grep -v _test.go   # exactly one mount, inside r.Group
git diff --stat -- . ':!plan/'   # api/httpapi only
```

## Metadata

architectural_impact: true

## Assumptions

- Task 7.2 has landed. Task 7.4 edits only `api/service` and may run at the same time.
