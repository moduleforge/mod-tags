# EntityResolver masking audit — mod-tags

## Purpose and scope

Records the direct source investigation the manager asked for before scoping the
masking task: does mod-tags use `core-api/entity.Resolver` today, at how many sites, and
how many entity-lookup sites bypass it. Grounds
[`phase-01-go-apiresp-migration/002-entityresolver-masking-adoption.md`](../phase-01-go-apiresp-migration/002-entityresolver-masking-adoption.md).

## Bottom line

mod-tags is a **partial-integration** module (like mod-tasks, unlike net-new mod-contacts):
it already imports and uses `core-api/entity.Resolver`, but only **1 of ~8 external-UUID
lookup sites** currently routes through it. The masking task is a **partial extension to full
coverage**, not a fresh wire-in.

## Integration today

- `api/service/service.go` injects `entityResolver *entity.Resolver` into `TagService`
  (constructor `New(...)`, field at `tag.go:98`).
- Exactly one live `entityResolver.Resolve(...)` call: `GetByUUID` at `tag.go:250`,
  `s.entityResolver.Resolve(ctx, coreQ, entityUUID, "tag")`. On a miss it returns
  `entity.ErrForbidden` (masking existence → intended 403).

## The `AllowNotFound` comment (manager-requested read)

- `tag.go:247-249` (in `GetByUUID`): *"Resolve UUID → internal entity ID. The default
  not-found policy returns ErrForbidden (masking existence); apps can opt this resource into
  404 via EntityResolver.AllowNotFound."*
- `tag_test.go:381-387` (`TestTagService_Get_NotFound`): *"Default EntityResolver policy:
  returns ErrForbidden when the UUID is not found, masking entity existence (privacy default
  per Phase E). Resources opting into 404 transparency (e.g. via AllowNotFound(\"tag\")) would
  return ErrNotFound; tags has not opted in."* The test asserts `errors.Is(err, entity.ErrForbidden)`.

**Interpretation:** the module's documented masking *intent* is exactly the design doc's
masking-by-default policy — `ErrForbidden` default, `AllowNotFound` as a per-resource
composition-root opt-in, and tags has *not* opted in. No repo in the ecosystem has a live
`AllowNotFound()` call site. Full masking adoption is the correct, intent-aligned direction.

## Site-by-site audit

External-UUID lookups (client-supplied UUID → internal ID; masking-relevant):

| # | Method | Site | Current miss behaviour | Target |
|---|--------|------|------------------------|--------|
| 1 | `GetByUUID` | `tag.go:250` via `Resolve` | `entity.ErrForbidden` (403) — **already masked** | keep (canonical sentinel after Wave 0) |
| 2 | `Create` | `tag.go:160` `GetEntityByUUID(subject)` | `ErrNotFound` → 404 | route through `Resolve` → 403 |
| 3 | `ListBySubject` | `tag.go:393` `GetEntityByUUID(subject)` — the `GET /entities/{uuid}/tags` keyed param | `ErrNotFound` → 404 | route through `Resolve` → 403 |
| 4 | `UpdateByUUID` | `tag.go:476` `GetTagByEntityUUID` (in-tx) | `ErrNotFound` → 404 | resolve entity UUID → 403 on miss |
| 5 | `UpdateTagValue` | `tag.go:556` `GetTagByEntityUUID` (in-tx) | `ErrNotFound` → 404 | resolve entity UUID → 403 on miss |
| 6 | `DeleteByUUID` | `tag.go:619` `GetTagByEntityUUID` (in-tx) | `ErrNotFound` → 404 | resolve entity UUID → 403 on miss |
| 7 | `Search` | `tag.go:323` `GetEntityByUUID(owner filter)` | returns empty `[]Tag{}` (non-leaking) | route through `Resolve` → 403 (see note) |
| 8 | `Search` | `tag.go:334` `GetEntityByUUID(subject filter)` | returns empty `[]Tag{}` (non-leaking) | route through `Resolve` → 403 (see note) |

NOT masking-relevant (internal `GetEntityByID` by int64 ID for owner/subject hydration —
these take an internal ID that came from an already-authorized row, so they cannot leak
external existence): `tag.go` lines 169, 271, 275, 359, 363, 434, 500, 504, 575, 579, 630, 634.
Leave unchanged.

## Latent bug uncovered (fixed incidentally by the response-layer task)

`GetByUUID` returns `entity.ErrForbidden` on a masked miss, but the current httpapi
`writeServiceErr` (`response.go:33-46`) only matches `service.ErrNotFound/ErrForbidden/
ErrInvalidInput/ErrConflict` — it does **not** match `entity.ErrForbidden`, so a masked GET
`/tags/{uuid}` miss currently falls through to `default → 500 internal_error`, not 403. This is
untested end-to-end: handler tests inject a fake service returning `service.*` sentinels, and
the service test (`tag_test.go:386`) only checks the service-layer error value. The Wave-0
migration to `apiresp.WriteError` — which recognises the canonical `apiresp.ErrForbidden`
(the resolver's return after Wave 0) via `errors.Is` — fixes this as a side effect. Called out
so the response-layer task adds an end-to-end assertion locking in 403.

## Search-filter nuance (flagged decision)

Sites 7–8 (`Search` owner/subject *filters*) already return an empty result set on a miss,
which is itself non-leaking (a caller cannot distinguish "entity absent" from "entity present
but no visible tags"). Converting them to `Resolve` → 403 follows the manager's "full coverage"
instruction and the masking-by-default policy, but it is a *behaviour change from "empty list"
to "403"* on these two filter params, not a 404→403 fix. The task carries this as an explicit
sub-decision; the 404→403 conversions (sites 2–6) are the unambiguous core.

## Slug semantics note for the implementer

`Resolver.Resolve(ctx, q, uuid, resourceSlug)` uses `resourceSlug` only as a key into its
`notFoundIs404` opt-in map — it does **not** validate the resolved entity's type. So the
subject-entity resolutions (sites 2, 3, 7, 8), whose target is an arbitrary entity type rather
than a `"tag"`, still resolve correctly; the slug only selects the masking policy bucket. Use a
slug that is (and stays) un-opted-in so misses mask to 403. Consider a distinct slug for
subject-reference resolutions (e.g. do not reuse `"tag"`) so a future `AllowNotFound("tag")`
opt-in on the tag resource does not unintentionally also un-mask subject lookups.
</content>
</invoke>
