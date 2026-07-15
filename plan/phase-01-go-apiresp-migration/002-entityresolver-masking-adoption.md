# Entityresolver Masking Adoption

## Purpose and scope

Extend mod-tags' existing partial `core-api/entity.Resolver` integration to **full masking
coverage**, so that a genuine entity miss on any external-UUID lookup returns `403 forbidden`
(existence-masking) via `EntityResolver.Resolve`, consistent with the masking-by-default policy in
[`docs/mf-standards/architecture/api-response-design.md`](../../docs/mf-standards/architecture/api-response-design.md)
(§"Existence masking"). This is a **partial extension** — mod-tags already masks `GetByUUID` via
the resolver; ~7 other lookup sites currently bypass it. Full site inventory and rationale:
[masking audit](../notes/masking-audit.md). Go-only, under `api/`. Sequenced **after** task 001.

## Requirements

Route every remaining external-UUID entity lookup in `api/service/tag.go` through
`s.entityResolver.Resolve(ctx, coreQ, <uuid>, <slug>)`, replacing the current direct
`GetEntityByUUID` / `GetTagByEntityUUID` miss handling. `Resolve` returns the canonical
`apiresp.ErrForbidden` on a genuine miss (Wave-0 behaviour), which `apiresp.WriteError` (installed
by task 001) maps to 403.

**Unambiguous 404 → 403 conversions (core of the task):**

1. **`Create`** — the client-supplied `SubjectEntityUUID` resolution at `tag.go:160-166` currently
   returns `ErrNotFound` (404) on miss. Resolve the subject UUID through `entityResolver.Resolve`
   (→ 403 on miss). The internal owner lookup (`GetEntityByID(actorEntityID)`, `tag.go:169`) is
   **not** a masking site and stays as-is.
2. **`ListBySubject`** — the subject UUID resolution at `tag.go:393-399` (the `GET
   /entities/{uuid}/tags` keyed param) currently returns `ErrNotFound` (404). Resolve through
   `entityResolver.Resolve` (→ 403).
3. **`UpdateByUUID`** — the in-tx `GetTagByEntityUUID` at `tag.go:476-482` returns `ErrNotFound`
   (404) on miss. Resolve the tag entity UUID up front (mirroring `GetByUUID`'s pre-tx
   `Resolve(ctx, coreQ, entityUUID, "tag")`), returning 403 on miss before entering the tx.
4. **`UpdateTagValue`** — same pattern as (3) for `tag.go:556-562`.
5. **`DeleteByUUID`** — same pattern as (3) for `tag.go:619-625`.

**Flagged empty-list → 403 conversions (secondary; see audit "Search-filter nuance"):**

6. **`Search` owner filter** (`tag.go:322-331`) and **subject filter** (`tag.go:333-342`) currently
   return an empty `[]Tag{}` on a filter-entity miss (already non-leaking). Per the manager's
   "full coverage" instruction, route these through `entityResolver.Resolve` so a genuine miss
   masks to 403. Implement this, but call it out in the task report as a deliberate behaviour change
   from "empty result" to "403" (distinct from the 404→403 fixes above) in case reviewers prefer to
   retain the empty-list behaviour for search filters.

**Cross-cutting implementation notes:**

- **Slug choice.** `Resolve`'s `resourceSlug` is only a key into the resolver's `AllowNotFound`
  opt-in map — it does not type-check the entity — so subject-reference resolutions (sites 1, 2, 6),
  whose target is an arbitrary entity type, resolve correctly. Use a slug that is (and stays)
  un-opted-in. Prefer a subject-specific slug (not `"tag"`) for subject-reference sites so a future
  `AllowNotFound("tag")` on the tag resource does not accidentally un-mask subject lookups. Keep
  `"tag"` for the tag-entity resolutions (sites 3–5, matching `GetByUUID`).
- **Post-resolve tag-row fetch.** After resolving the tag entity UUID in `UpdateByUUID`/
  `UpdateTagValue`/`DeleteByUUID`, the subsequent in-tx `GetTagByEntityUUID` should normally
  succeed. Decide and document the residual `pgx.ErrNoRows` behaviour for the edge case where the
  entity row exists (e.g. archived) but the live tag row is gone: prefer masking-consistent
  `apiresp.ErrForbidden` over `ErrNotFound` to avoid a masking gap, unless a clearer reason exists.
- Do **not** touch the internal `GetEntityByID(int64)` owner/subject hydration lookups — they take
  an already-authorized internal ID and cannot leak external existence.

**Test updates:**

- Update the service tests in `api/service/tag_test.go` that assert `ErrNotFound`/404 on a missing
  subject or tag (Create, ListBySubject, Update*, Delete) to expect the canonical forbidden
  sentinel. The mock `coreQuerier`/resolver must be driven so that a missing UUID surfaces the
  resolver's forbidden path.
- Update the handler tests in `api/httpapi/handlers_test.go` whose fake service returns
  `service.ErrNotFound` for a genuine miss (e.g. `TestHandleGetTag_404_*`,
  `TestHandlePatchTag_404_NotFound`, `TestHandleDeleteTag_404_Stranger`,
  `TestHandleSubjectTags_404_UnknownSubject`) to reflect the new masking status where the scenario
  represents a genuine entity miss. Preserve genuinely-distinct cases (e.g. a `403` that already
  represents an authz denial such as `TestHandlePutTag_403_SubjectTries`) unchanged. Carefully
  distinguish "entity does not exist" (now 403-masked) from "entity exists, authz denies" (already
  403) when updating expectations.

## Validation

- `cd api && go build ./...` and `cd api && go test ./...` pass.
- `grep -n "GetEntityByUUID\|GetTagByEntityUUID" api/service/tag.go` — every remaining
  external-UUID lookup that previously produced `ErrNotFound` now flows through
  `entityResolver.Resolve` (or a documented post-resolve residual), and no user-facing genuine miss
  returns 404 from the entity/tag lookup path.
- A service/handler test exists asserting **403** (masked) for a genuine miss on each of: `GetByUUID`
  (from task 001), `Create` (missing subject), `ListBySubject` (missing subject),
  `UpdateByUUID`/`UpdateTagValue`/`DeleteByUUID` (missing tag).
- The two `Search` filter-miss paths route through `Resolve` (or the report explicitly records the
  reviewer-facing decision to retain empty-list behaviour).
- No 404 remains reachable from an `EntityResolver`-eligible miss; router-level 404s (unknown
  routes) are unaffected.

## Metadata

architectural_impact: true

## Assumptions

- **Wave 0 is merged** and `core-api/entity.Resolver.Resolve` returns the canonical
  `apiresp.ErrForbidden`/`apiresp.ErrNotFound`. Task 001 (response-layer migration) is complete, so
  `apiresp.WriteError` maps the forbidden sentinel to 403 end-to-end. If either precondition is
  unmet, halt and report.
- No resource opts into `AllowNotFound` today, so all masked misses resolve to 403; this task does
  **not** add any `AllowNotFound` opt-in (that remains a composition-root, per-app decision).
- The `entityResolver` field is already injected into `TagService` (`service.go`), so no
  constructor/wiring change is needed.

## References

- [masking audit](../notes/masking-audit.md) — site-by-site inventory, the `AllowNotFound`
  comment/intent, slug semantics, and the search-filter nuance.
- [`docs/mf-standards/architecture/api-response-design.md`](../../docs/mf-standards/architecture/api-response-design.md)
  §"Existence masking (`not_found` vs `forbidden`)" and §"HTTP status mapping".
- Reference implementation pattern: the existing `GetByUUID` resolver call at `tag.go:250`, and
  mod-tasks' / mod-core's own `entityResolver.Resolve` usage.

## Checkpoint hints

- After converting the `Create` and `ListBySubject` subject resolutions.
- After converting the `UpdateByUUID`/`UpdateTagValue`/`DeleteByUUID` tag resolutions.
- After the flagged `Search` filter conversions.
- After updating service and handler tests.
