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

## Status

- **Outcome:** succeeded. Date: 2026-07-18.
- **Validation summary:** `cd api && go build ./...`, `go vet ./...`, and `go test ./...` all pass
  (39 service tests, 30 handler tests). `gofmt -l .` reports no formatting diffs. `grep -n
  "GetEntityByUUID\|GetTagByEntityUUID" api/service/tag.go` shows zero remaining direct
  `GetEntityByUUID` calls and four `GetTagByEntityUUID` call sites, all of which are documented
  post-resolve residual fetches (see notes below).
- **Affected source files:**
  - `api/service/tag.go` — all six requirement sites converted.
  - `api/service/tag_test.go` — six existing tests updated to assert `ErrForbidden`; two new tests
    added (`TestTagService_Search_OwnerFilterMasked`, `TestTagService_Search_SubjectFilterMasked`).
  - `api/httpapi/handlers_test.go` — four tests renamed and updated to assert 403 instead of 404
    (`TestHandleGetTag_403_Unauthorized`, `TestHandlePatchTag_403_NotFound`,
    `TestHandleDeleteTag_403_Stranger`, `TestHandleSubjectTags_403_UnknownSubject`).
- **Requirements 1–5 (unambiguous 404→403 conversions):** all converted as specified.
  `Create`/`ListBySubject` use a shared `"subject_entity"` slug (distinct from `"tag"`, per the
  cross-cutting slug-choice note). `UpdateByUUID`/`UpdateTagValue`/`DeleteByUUID` resolve the tag
  entity UUID pre-tx via `entityResolver.Resolve(ctx, coreQ, entityUUID, "tag")`, mirroring
  `GetByUUID`. The Resolve call was placed *after* the existing pre-fetch `Authorize(ctx, op, nil)`
  call in these three methods (unchanged authorize-target semantics — restructuring authorization to
  be target-aware was not requested and was left alone) but *before* `txhelper.Run`, satisfying "up
  front... before entering the tx." Either order (resolve-then-authorize or
  authorize-then-resolve) yields 403 in every current test scenario since both paths converge on the
  same status; this task did not change the authorize call's target.
- **Requirement 6 (Search filter conversions — deliberate behaviour change):** implemented as
  specified. The owner filter uses slug `"owner_entity"`, the subject filter uses `"subject_entity"`
  (same slug as sites 1–2, since it is the same semantic role). **Flagging for reviewers per the
  task doc's explicit instruction:** this changes `Search`'s owner/subject-filter-miss behaviour from
  returning an empty `[]Tag{}` (already non-leaking) to returning `ErrForbidden` (403). New tests
  `TestTagService_Search_OwnerFilterMasked`/`_SubjectFilterMasked` lock in the new behaviour. If
  reviewers prefer to retain the old empty-list behaviour for these two filter params specifically,
  that is a one-line revert per filter (swap the `Resolve` call back to `GetEntityByUUID` +
  `pgx.ErrNoRows` → `return []Tag{}, nil`).
- **Post-resolve tag-row fetch residual (cross-cutting note):** for `UpdateByUUID`/`UpdateTagValue`/
  `DeleteByUUID`, the decision was made per the task doc's stated preference: the residual
  `pgx.ErrNoRows` from the in-tx `GetTagByEntityUUID` (reached only in the data-consistency edge
  case where the entity resolved but the live tag row is gone) now returns `ErrForbidden`, not
  `ErrNotFound` — masking-consistent, avoiding a residual gap. `GetByUUID`'s own equivalent
  post-resolve fetch (`tag.go:263`) was deliberately **left unchanged** (still returns `ErrNotFound`
  on this residual case): `GetByUUID`/site 1 is explicitly out of this task's Requirements 1–6 (the
  masking audit categorizes it "already masked... keep"), so no source change was made there. See
  `flagged_for_manager` in the task-agent's structured report for a note on this residual
  inconsistency.
- **List-route masking-consistency followup:** `ListBySubject`'s subject-entity resolution now
  routes through `entityResolver.Resolve` before authorization, so an inaccessible/nonexistent
  subject entity masks to 403 exactly like a direct lookup (`GetByUUID`) — this satisfies the
  "Verify list-route masking consistency" followup in `plan/followups.yaml`.
- **Local build environment:** built via a git-ignored worktree-root `go.work` (per the dispatch's
  known-environment-issue guidance). Mid-session, an unrelated concurrent merge landed on
  `mod-core`'s live `main` branch (an `apps`/`GetAppBySlug` addition to the `core-model` `Querier`
  interface) that broke this worktree's build by requiring a method mod-tags' test mocks don't
  implement. Worked around by pinning `go.work`'s replace directives to a detached `mod-core`
  worktree at commit `4557ba4` (the mainline state immediately before that merge) instead of the
  live, moving `mod-core/main` checkout. This is a local build aid only (gitignored, not a source
  change) — no mod-tags source or mock files were touched to address the drift.
- **Assumptions applied:** all three `## Assumptions` held as stated — Wave 0 (task 001) is merged
  into this branch's history, `apiresp.WriteError` maps the canonical `apiresp.ErrForbidden` sentinel
  to 403 end-to-end, no resource opts into `AllowNotFound` today, and the `entityResolver` field was
  already wired into `TagService` (no constructor change needed).
