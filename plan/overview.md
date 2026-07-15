# mod-tags API response & error standardization (Wave 2)

## Purpose and scope

Migrate mod-tags' HTTP API surface and its GUI error handling onto the ecosystem-wide
API-response contract defined in
[`docs/mf-standards/architecture/api-response-design.md`](../docs/mf-standards/architecture/api-response-design.md)
(the submodule-mounted design doc, read in full for this plan). This is **Wave 2** of the
multi-phase standardization effort, sequenced after Wave 1 (mod-authz / mod-users / mod-tasks /
mod-audit).

In scope (mod-tags repo only — `api/` and `gui/`, not `model/`):

1. **Go response-layer migration** — replace the copy-pasted `jsonOK`/`jsonErr`/`writeServiceErr`
   trio in `api/httpapi/response.go` with `apiresp.WriteJSON`/`apiresp.WriteError`; re-home the
   four local `api/service/errors.go` sentinels as aliases to the canonical `apiresp` sentinels;
   and convert the direct handler-level error writes to the canonical error-code vocabulary
   (`unauthenticated` not `unauthorized`, `invalid_input` not `bad_request`).
2. **Go full EntityResolver masking adoption** — extend the existing partial resolver integration
   (currently 1 of ~8 external-UUID lookup sites) to full coverage, so a genuine entity miss masks
   to `403 forbidden` via `EntityResolver.Resolve` rather than leaking `404 not_found`. See the
   [masking audit](./notes/masking-audit.md).
3. **GUI error-handling migration** — replace the hand-rolled `fetch`/error-parsing in
   `gui/src/lib/api.ts` with `@moduleforge/core-gui`'s shared typed `request()` wrapper, then
   migrate the ~8 ad-hoc `err instanceof Error ? err.message : '…'` sites across `TagEditor.tsx`
   and `TagList.tsx` onto `useApiError` + `<FieldError>`/`<ErrorBanner>` (and the Toast provider
   for toast-worthy failures).
4. **Documentation updates** — reconcile the module's own docs (`AGENTS.md`, `next-steps.md`,
   etc.) with the new public error-envelope shape, canonical codes, and masking behaviour.

Explicitly **out of scope**: any change to any other module repo; the design doc itself; adding
mod-tags to any app's manifest; standardizing the `GET /entities/{uuid}/tags` `{tags: […]}` list
envelope to a bare array (a known pre-existing deviation — see [Known constraints](#known-constraints-and-flags)).

## Current status

**Not started. Hard-blocked on Wave 0 (`mod-core` plan `apiresp-error-widgets`), which is NOT
YET MERGED.** Implementation of every task below cannot begin until Wave 0 lands on `mod-core`,
because both the Go and GUI work import artifacts Wave 0 builds:

- **Go:** the `github.com/moduleforge/core-api/apiresp` package — sentinels
  `ErrUnauthenticated`/`ErrForbidden`/`ErrNotFound`/`ErrInvalidInput`/`ErrConflict`,
  `WriteJSON(w, status, v)`, `WriteError(w, r, err)`, `InvalidInput(...)`, and field-error types
  — plus the change that makes `core-api/entity.Resolver.Resolve` return the canonical
  `apiresp.ErrForbidden`/`apiresp.ErrNotFound`.
- **GUI:** `@moduleforge/core-gui` exports — the wire types (`FieldError`, `ApiError`,
  `ApiErrorResponse`), `ApiRequestError`, the shared typed `request()` fetch wrapper,
  `<FieldError>`, `<ErrorBanner>`, the Toast provider + `useToast`, and the `useApiError` hook.

Every task document restates this precondition in its `## Assumptions`. Because Wave 0 is
unmerged, the exact exported symbol names/signatures used below are drawn from the **design-doc
contract**, and each task instructs the implementer to reconcile them against Wave 0's actual API
at implementation time.

The plan begins with **Phase 01 (Go)** and **Phase 02 (GUI)**, which are fully independent tracks
(disjoint file sets, each gated only on Wave 0) and may run in parallel. **Phase 03 (docs)** runs
last, after both.

## Overview

### Phase 01 — Go apiresp migration (`api/`)

Two tasks, run in sequence (both touch `api/service/tag.go` and the Go test suite; the masking
task also relies on the canonical error mapping the first task installs):

- **001 — Response-layer migration.** Swap `response.go`'s trio for `apiresp.WriteJSON`/
  `WriteError`; re-home `service/errors.go`'s four sentinels as aliases to the `apiresp`
  canonical set; convert all ~26 direct `jsonErr(...)` handler call sites (in `tags.go` and
  `subject_tags.go`) and the `jsonOK(...)` calls to `apiresp`, mapping `unauthorized`→
  `unauthenticated` (401) and `bad_request`→`invalid_input` (400). Update the Go tests for the
  new **nested** error envelope (`{"error":{"code","message","details?"}}`) and canonical codes,
  and add an end-to-end assertion that a masked `GET /tags/{uuid}` miss now returns **403**
  (fixing the latent `entity.ErrForbidden`→500 bug documented in the audit). `architectural_impact: true`.
- **002 — EntityResolver masking adoption.** Extend `entityResolver.Resolve` to every remaining
  external-UUID lookup site so a genuine miss masks to 403 (see the
  [masking audit](./notes/masking-audit.md) table): the unambiguous 404→403 conversions in
  `Create`, `ListBySubject`, `UpdateByUUID`, `UpdateTagValue`, `DeleteByUUID`, plus the
  flagged empty-list→403 conversion of the two `Search` filter params. Update service and handler
  tests to expect 403 on genuine misses. `architectural_impact: true`.

### Phase 02 — GUI error handling (`gui/`)

Two tasks, run in sequence (the components consume the typed client from the first task):

- **001 — Adopt the shared `request()` client.** Rewrite `gui/src/lib/api.ts` to use
  `@moduleforge/core-gui`'s `request()` wrapper and `ApiRequestError`/wire types in place of the
  bespoke `handleResponse`/`throw new Error(message)` logic, so the client parses the nested error
  envelope and throws typed `ApiRequestError`s. Promote `@moduleforge/core-gui` from an optional
  peer dependency to a real one. Keep `listBySubject` extracting `.tags` from the existing server
  response shape (the list-envelope deviation is out of scope).
- **002 — Migrate ad-hoc error sites onto the widget set.** Replace the ~8 hand-rolled catch
  blocks and inline `<span role="alert">`/`submitError` state in `TagEditor.tsx` and `TagList.tsx`
  with `useApiError` + `<FieldError>`/`<ErrorBanner>` (and dispatch toast-worthy failures to the
  Toast provider). Update `gui/src/lib/mockClient.ts` and the affected `*.stories.tsx` to throw
  `ApiRequestError`s so the workbench exercises the new surfaces.

### Phase 03 — Documentation Updates

- **001 — Update architecture/module docs.** Reconcile mod-tags' own documentation with the new
  public API-response behaviour introduced by Phase 01. mod-tags has no `docs/architecture.md` and
  no `docs/*-spec.md`; the review therefore targets `AGENTS.md`, `next-steps.md`, `README.md`, and
  `docs/decisions/`, correcting stale statements (notably `next-steps.md`'s smoke-test
  expectations that assume `404` for missing tags, now `403` under masking, and its documented
  error/route behaviour). Runs after Phases 01 and 02 land.

### Parallelism and dependencies

- **Phase 01 ‖ Phase 02** — independent tracks (disjoint `api/` vs `gui/` file sets, each gated
  only on Wave 0). Their heads —
  `phase-01-go-apiresp-migration/001-response-layer-migration.md` and
  `phase-02-gui-error-handling/001-adopt-shared-request-client.md` — may start concurrently.
- **Within each phase**, task 002 depends on task 001.
- **Phase 03** depends on Phase 01 (and reviews any GUI-doc implications from Phase 02).
- **Semantic coupling note:** Phase 01 makes the server emit the nested error envelope and Phase
  02 makes the client parse it; both must land for a real deployment to be self-consistent. Since
  mod-tags is wired into no app today, there is no live deployment to keep consistent mid-flight,
  and GUI validation runs against the mock client / Ladle workbench, so the two tracks remain
  execution-independent.

### Validation posture

mod-tags is currently wired into **no** app (`app-mfdemo`, `app-mftodo`, and the other app
manifests do not include it), so there is **no app-level regeneration/verification step** for this
plan — unlike some Wave-1 plans. Validation is limited to:

- **Go:** `make test` / `cd api && go test ./...` and `cd api && go build ./...`.
- **GUI:** `cd gui && bun run typecheck`, `cd gui && bun run build`, and the Ladle workbench
  (`cd gui && bun run dev`, or `make preview`) — not a real composed app.

### Known constraints and flags

- **List-envelope deviation (out of scope, flagged).** `GET /entities/{uuid}/tags` returns
  `{"tags": […]}` while `GET /tags` returns a bare array; the design doc characterizes mod-tags as
  a bare-array flat-family module. This known asymmetry (see `next-steps.md`) is left intact — the
  GUI client keeps extracting `.tags` — because standardizing the wire shape is a breaking change
  the manager did not scope. Recommended as a follow-up.
- **Wave-0 symbol reconciliation.** Exact `apiresp` and `@moduleforge/core-gui` export
  names/signatures must be confirmed against Wave 0's merged code before/at implementation.
</content>
