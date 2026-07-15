# Response Layer Migration

## Purpose and scope

Replace mod-tags' copy-pasted HTTP response/error trio and its locally-defined service sentinels
with the shared `github.com/moduleforge/core-api/apiresp` package (built by Wave 0), and bring the
module onto the canonical error-envelope shape and error-code vocabulary defined in
[`docs/mf-standards/architecture/api-response-design.md`](../../docs/mf-standards/architecture/api-response-design.md).

This is the mod-tags application of the same "dogfood" migration mod-core performs on itself. It is
a **Go-only** task under `api/`. Invoke the standard Go implementation skill.

## Requirements

1. **Re-home the service sentinels as aliases.** In `api/service/errors.go`, replace the four
   independently-declared sentinels (`ErrNotFound`, `ErrForbidden`, `ErrInvalidInput`,
   `ErrConflict`) with aliases to the canonical `apiresp` set so existing `errors.Is` /
   `fmt.Errorf("%w: …", service.ErrX)` call sites throughout `api/service/` keep compiling
   unchanged:
   ```go
   var (
       ErrNotFound     = apiresp.ErrNotFound
       ErrForbidden    = apiresp.ErrForbidden
       ErrInvalidInput = apiresp.ErrInvalidInput
       ErrConflict     = apiresp.ErrConflict
   )
   ```
   (Confirm against Wave 0 whether `ErrUnauthenticated` should also be surfaced here; the service
   layer does not currently return a 401 sentinel — the missing-actor 401 is decided at the handler
   layer — so re-homing the four above is the baseline.)
2. **Delete the local response trio.** Remove `jsonOK`, `jsonErr`, and `writeServiceErr` from
   `api/httpapi/response.go` (the whole file, or reduce it to nothing but a package doc). All
   status/code/envelope decisions now come from `apiresp`.
3. **Migrate success writes.** Replace every `jsonOK(w, status, body)` call in `api/httpapi/tags.go`
   and `api/httpapi/subject_tags.go` with `apiresp.WriteJSON(w, status, body)`. Preserve existing
   status codes (201 on create, 200 on read/update/search/list, 204 on delete) and the existing
   response bodies **as-is** — including the `map[string]any{"tags": resp}` body of
   `handleSubjectTags` (the list-envelope deviation is out of scope for this plan).
4. **Migrate service-error writes.** Replace every `writeServiceErr(w, err)` call with
   `apiresp.WriteError(w, r, err)` (note the added `*http.Request` argument, available in each
   handler). `WriteError` maps the canonical sentinels via `errors.Is` and emits the nested
   envelope; the per-module mapping switch disappears.
5. **Migrate handler-level direct error writes to canonical codes.** The handlers currently call
   `jsonErr(...)` directly for input/auth failures with **non-canonical** codes. Convert each to
   `apiresp`, adopting the canonical vocabulary the design doc mandates:
   - Missing actor → currently `jsonErr(w, 401, "unauthorized", …)` in all seven handlers
     (`tags.go` lines ~52, ~94, ~151, ~186, ~248, ~292; `subject_tags.go` ~16). Emit the canonical
     **`unauthenticated` (401)** via `apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)` (or the
     Wave-0 idiom for a bare-sentinel write).
   - Malformed body / bad UUID / missing-required-field → currently `jsonErr(w, 400, "bad_request",
     …)` (many sites in `tags.go`: JSON decode, `uuid.Parse`, absent `color`, absent `value`, etc.).
     Emit the canonical **`invalid_input` (400)**, e.g.
     `apiresp.WriteError(w, r, fmt.Errorf("%w: <message>", apiresp.ErrInvalidInput))` or via
     `apiresp.InvalidInput(...)` where a field-level detail is warranted. Preserve the existing
     human-readable messages.
   Confirm the exact bare-sentinel-write idiom against Wave 0's `apiresp` API.
6. **Update the Go tests** for the new contract:
   - Any handler test that decodes the error body must expect the **nested** shape
     `{"error":{"code":"…","message":"…"}}` rather than the old flat `{"error":"…","message":"…"}`.
     (Most handler tests in `api/httpapi/handlers_test.go` assert only on status codes; the one
     body assertion, `TestHandleSubjectTags` checking `body["tags"]`, is on a success body and is
     unaffected.)
   - Any test asserting the old codes `"unauthorized"`/`"bad_request"` must move to
     `"unauthenticated"`/`"invalid_input"`.
   - The service-layer test `TestTagService_Get_NotFound` (`api/service/tag_test.go:386`) currently
     asserts `errors.Is(err, entity.ErrForbidden)`. After Wave 0 the resolver returns the canonical
     `apiresp.ErrForbidden`; update the assertion to the canonical sentinel (or the re-homed
     `service.ErrForbidden` alias) as appropriate to Wave 0's aliasing.
   - **Add an end-to-end handler assertion** that a masked `GET /tags/{uuid}` miss (service returns
     the canonical forbidden sentinel) now yields **403**, locking in the fix for the latent
     `entity.ErrForbidden`→500 bug described in the [masking audit](../notes/masking-audit.md).
7. **No behavioural change to success bodies or status codes** beyond the error-envelope shape and
   the two code renames. The masking status-code changes (404→403) belong to task 002, not here.

## Validation

- `cd api && go build ./...` succeeds; `api/httpapi/response.go` no longer defines `jsonOK`/
  `jsonErr`/`writeServiceErr`.
- `grep -rn "jsonOK\|jsonErr\|writeServiceErr" api/` returns no call sites (only, at most, removed
  definitions) — every usage is migrated to `apiresp`.
- `grep -rn '"unauthorized"\|"bad_request"' api/` returns nothing (canonical codes adopted).
- `cd api && go test ./...` passes, including the new 403-on-masked-miss end-to-end assertion and
  the nested-envelope body assertions.
- `api/service/errors.go` declares the four sentinels as aliases to `apiresp.*` (a grep confirms
  `= apiresp.Err`), and existing `service.ErrX` references across `api/service/` still compile.
- Manual scan: every handler error path is `apiresp.WriteError(w, r, err)` or an `apiresp`
  sentinel/`InvalidInput` write; no map-literal error bodies remain.

## Metadata

architectural_impact: true

## Assumptions

- **Wave 0 (`mod-core` plan `apiresp-error-widgets`) is merged.** This task cannot start otherwise.
  It requires `github.com/moduleforge/core-api/apiresp` to exist and export `WriteJSON`,
  `WriteError(w, r, err)`, `InvalidInput(...)`, and the `ErrUnauthenticated`/`ErrForbidden`/
  `ErrNotFound`/`ErrInvalidInput`/`ErrConflict` sentinels, and the `core-api/entity.Resolver` to
  return the canonical `apiresp` sentinels. If Wave 0 is not merged, halt and report.
- The exact `apiresp` symbol names/signatures are taken from the design-doc contract; reconcile
  against Wave 0's actual API before coding.
- The `GET /entities/{uuid}/tags` `{tags: […]}` body is intentionally left unchanged.

## References

- [`docs/mf-standards/architecture/api-response-design.md`](../../docs/mf-standards/architecture/api-response-design.md)
  — canonical envelope, error-code vocabulary, sentinel→status mapping, and Go-layer ownership
  (§"Go-layer ownership", §"Error-code vocabulary", §"HTTP status mapping").
- [masking audit](../notes/masking-audit.md) — documents the latent `entity.ErrForbidden`→500 bug
  this task's end-to-end assertion locks down.
- Current code: `api/httpapi/response.go` (trio to delete), `api/httpapi/tags.go` +
  `api/httpapi/subject_tags.go` (`jsonOK`/`jsonErr` call sites), `api/service/errors.go` (sentinels
  to re-home), `api/httpapi/handlers_test.go` + `api/service/tag_test.go` (tests to update).

## Checkpoint hints

- After re-homing `api/service/errors.go` sentinels (confirm `api/service` still builds/tests).
- After deleting the trio and migrating `api/httpapi/tags.go`.
- After migrating `api/httpapi/subject_tags.go` and the handler-level canonical-code conversions.
- After updating tests (nested envelope, canonical codes, 403-on-masked-miss assertion).
