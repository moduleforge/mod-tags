# Restore Test Build

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

Make the baseline green before any code task. `go vet ./...` and `go test ./...` in `api/` fail to build `api/httpapi` and `api/service`, because both test fakes that assert `coredb.Querier` lack 11 methods mod-core `main` added. Test-only fix; see the [baseline note](../notes/handler-inventory.md#building-and-validating-from-a-task-worktree).

## Requirements

1. Add these 11 methods to `fakeCoreQuerier` (`api/httpapi/mock_test.go`, asserted at `:157`) and `mockCoreQuerier` (`api/service/mock_test.go`, asserted at `:407`): `GetFieldCryptoKeyByVersion`, `InsertActiveFieldCryptoKey`, `InsertInitialFieldCryptoKey`, `ListFieldCryptoKeyMetadata`, `ListUsableFieldCryptoKeys`, `MarkFieldCryptoKeyCompromised`, `RetireActiveFieldCryptoKey`, `SetFieldCryptoKeyDecryptableUntil`, `UpdateCorporationEINBlob`, `UpdateNaturalPersonSSNBlob`, `UpdateServiceAccountLabel`.
   - Copy the exact signatures from `/Users/zane/playground/moduleforge/mod-core/model/db/querier.go`.
   - Follow each file's pattern for uncalled methods (zero-value return). Do not panic.
2. Grep every other `_test.go` file in `api/`, including `//go:build integration` files, for types asserted against `coredb.Querier`, and fix any that fail the same way.
3. No non-test changes. No `go.mod`/`go.sum` edits.

## Validation

From `api/` ([build setup](../notes/handler-inventory.md#building-and-validating-from-a-task-worktree)):

```bash
test -z "$(gofmt -l .)" || gofmt -l .
go vet ./...
go vet -tags=integration ./...
go test ./...
git diff --stat -- . ':!plan/'   # only the two mock_test.go files (plus any file found in step 2)
```

## Metadata

architectural_impact: false

## Assumptions

- Runs in parallel with task 7.1 (read-only). The planner verified with a `go vet -overlay` that these 11 stubs are the only baseline failure.

## Status

- Outcome: succeeded (2026-10-11).
- Changed files: `api/httpapi/mock_test.go` (11 stubs added to `fakeCoreQuerier`), `api/service/mock_test.go` (11 stubs added to `mockCoreQuerier`). No other file changed; no non-test or `go.mod`/`go.sum` edits.
- Requirement 2 sweep: grepped every `_test.go` under `api/` (including `//go:build integration` files) for `coredb.Querier` use. No other type failed to satisfy the interface; `go vet` with and without `-tags=integration` confirms it.
- Validation: `gofmt -l .` clean; `go vet ./...` passed; `go vet -tags=integration ./...` passed; `go test ./...` passed (`httpapi`, `service`). `git diff --stat -- . ':!plan/'` lists only the two mock files.
- Integration run (extra, not in `## Validation`): `go test -tags=integration -p 1 -count=1 -v ./service/...` exited 0, but the grant suite printed `integration: skipping tag grant tests — docker inspect: exit status 1` and no tests ran, so integration coverage is skipped, not passed.
- Logs: `<worktree>/.flow/validation-logs/` (`02-go-vet.log`, `03-go-vet-integration.log`, `04-go-test.log`, `05-go-test-integration.log`).
- Decisions under Assumptions: the planner's overlay finding held; no further baseline failures appeared. In `service/mock_test.go` the `GetFieldCryptoKeyByVersion` stub returns `pgx.ErrNoRows` to match that file's `Get*` stub pattern; all other stubs return zero values with a nil error.
