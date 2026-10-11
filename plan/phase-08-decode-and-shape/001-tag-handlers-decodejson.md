# Tag Handlers DecodeJSON and Stateless-First Service

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

Move the 3 mod-tags body routes to `apiresp.DecodeJSON` (they read bodies with no size cap today), adopt `apiresp.ParseUUIDParam` for every UUID input, and move `TagService.Create`'s and `TagService.Search`'s stateless validation ahead of their authorize calls. Decisions D5, D6 and D7 in the [handler inventory](../notes/handler-inventory.md#planning-decisions) apply; helper usage is in [common section 3](/Users/zane/playground/moduleforge/mod-core/plan/waves/http-error-precedence-conformance/common-conformance-instructions.md#3-helper-reference-final-signatures).

## Requirements

1. `handleCreateTag` (`tags.go:54`), `handlePutTag` (`:188`), `handlePatchTag` (`:250`): replace the `json.NewDecoder` blocks with `apiresp.DecodeJSON(w, r, &req, apiresp.WithCodePrefix("tags"))`. Keep the request structs and the handler-level checks that follow decode (PUT color required/type, PATCH value required), with their current errors.
2. `ParseUUIDParam` (D6): path `{uuid}` on `GET`/`PUT`/`PATCH`/`DELETE /tags/{uuid}` and `GET /entities/{uuid}/tags` (`apiresp.ParseUUIDParam("uuid", chi.URLParam(r, "uuid"))`); query `owner` and `subject` on `GET /tags`; query `scope` on `GET /tag-templates`; body `subject` on `POST /tags`. Each failure is written with `apiresp.WriteError` and returns before any service call.
3. `TagService.Create` (`api/service/tag.go:128`): move purpose/value trim and validation and the color format check (`:159-175`) ahead of `s.az.Authorize` (`:154`), after task 7.4's actor guard. `TagService.Search` (`:304`): move "at least one of owner or subject is required" (`:331`) ahead of `Authorize` (`:327`). Errors unchanged.
4. Tests (prefix `tgShape`):
   - with a denying authorizer, `Create` with a blank purpose/value or bad color, and `Search` with neither filter, return the 400 error, not the 403, and no `Authorize` call is recorded;
   - through `NewRouter`: over-cap body gives 400 with `tags.body_too_large`; trailing data and unknown fields give `tags.invalid_json_body`; a malformed path UUID gives an `invalid_uuid` detail with field `uuid`; a malformed `owner` query gives field `owner`;
   - existing tests keep passing (they assert status, not details; if one asserted the absence of details, update it and say so).

   Show one new test red before the change.
5. Append the changes to the [wire-risk note](../notes/wire-risk.md#recorded-by-tasks).

## Validation

From `api/` ([build setup](../notes/handler-inventory.md#building-and-validating-from-a-task-worktree)):

```bash
test -z "$(gofmt -l .)" || gofmt -l .
go vet ./...
go vet -tags=integration ./...
go test ./...
grep -rn 'json.NewDecoder(r.Body)\|uuid.Parse(' httpapi/ | grep -v _test.go   # no hits
grep -c 'apiresp.DecodeJSON' httpapi/tags.go   # 3, each with WithCodePrefix("tags")
```

Manual read: in `TagService.Create` and `.Search`, `Authorize` follows every validation return.

## Metadata

architectural_impact: false

## Assumptions

- Phase 07 has landed.

## Checkpoint hints

- After the handler changes with the suite green.
- After the service reorder and its tests.
