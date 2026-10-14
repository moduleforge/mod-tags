# Remove Inline Authn Checks

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

Delete the 8 sudo-unaware inline `opctx.ActorEntityID` checks (`tags.go` lines 55, 97, 154, 189, 251, 295; `subject_tags.go:17`; `tag_templates.go:48`), superseded by task 7.3's `RequireAuth` mount (common rule 1; D2 in the [handler inventory](../notes/handler-inventory.md#planning-decisions)).

## Requirements

1. Confirm task 7.3's tests cover all 8 routes with anonymous callers getting 401 through the middleware. If not, halt and report.
2. Delete the 8 check blocks and the now-unused `opctx` imports. Update the `handleListTagTemplates` doc comment: authentication comes from the router; still no per-row authorization.
3. The existing `*_401_Unauthenticated` tests use `NewRouter`, so they must keep passing unchanged; if one fails, halt and report.
4. Add a test (prefix `tgInl`) through `NewRouter` that a sudo-only identity is not 401 on any of the 8 routes (it reaches the fake service).
5. Append the sudo-only entry to the [wire-risk note](../notes/wire-risk.md#recorded-by-tasks).

## Validation

From `api/` ([build setup](../notes/handler-inventory.md#building-and-validating-from-a-task-worktree)):

```bash
test -z "$(gofmt -l .)" || gofmt -l .
go vet ./...
go test ./...
grep -rn 'ActorEntityID\|ErrUnauthenticated' httpapi/ | grep -v _test.go   # no hits
```

## Metadata

architectural_impact: false

## Assumptions

- Tasks 7.3 and 7.4 have landed.

## Status

- Outcome: succeeded (2026-10-11).
- Requirement 1: confirmed. `api/httpapi/router_authn_test.go` covers all 8 routes with anonymous callers (well-formed and malformed variants) through both `NewRouter` and `RegisterRoutes`; all expect 401.
- Requirement 2: the 8 inline `opctx.ActorEntityID` checks are deleted (`api/httpapi/tags.go` x6, `subject_tags.go`, `tag_templates.go`); the now-unused `opctx` imports are removed; the `handleListTagTemplates` doc comment now states authentication comes from the router and that there is still no per-row authorization.
- Requirement 3: the existing `*_401_Unauthenticated` tests use `NewRouter` and pass unchanged.
- Requirement 4: added `api/httpapi/router_sudo_only_test.go` (`TestTgInl_NewRouter_SudoOnlyIsNotUnauthenticated`, 8 subtests) asserting a sudo-only identity is not 401 and gets 2xx from the fake service on each route.
- Requirement 5: sudo-only entry appended to `plan/notes/wire-risk.md` under Recorded by tasks.
- Validation (from `api/`): `gofmt -l .` clean; `go vet ./...` and `go vet -tags=integration ./...` pass; `go test ./...` passes; `grep -rn 'ActorEntityID\|ErrUnauthenticated' httpapi/ | grep -v _test.go` has no hits. Logs: `<worktree>/.flow/validation-logs/05-*.log`.
- Integration suites (`service/*_integration_test.go`) were not run; this task changes only `httpapi/`.
- Source files: `api/httpapi/tags.go`, `api/httpapi/subject_tags.go`, `api/httpapi/tag_templates.go`, `api/httpapi/router_sudo_only_test.go`.
