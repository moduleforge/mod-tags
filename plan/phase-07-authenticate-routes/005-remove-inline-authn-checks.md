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
