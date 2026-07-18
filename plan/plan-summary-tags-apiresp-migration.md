# Session summary: mod-tags API response & error standardization (Wave 2)

Plan slug: `tags-apiresp-migration`

## What was planned and why

This plan is **Wave 2** of an ecosystem-wide, multi-phase effort to standardize every module's
HTTP API surface and GUI error handling onto a single API-response contract (defined in
`docs/mf-standards/architecture/api-response-design.md`), sequenced after Wave 1 (mod-authz /
mod-users / mod-tasks / mod-audit) and hard-blocked on Wave 0 (`mod-core` plan
`apiresp-error-widgets`) landing first.

Scope was confined to the mod-tags repo (`api/` and `gui/`, not `model/`), covering four areas:

1. **Go response-layer migration** — replace mod-tags' copy-pasted `jsonOK`/`jsonErr`/
   `writeServiceErr` trio with the shared `apiresp.WriteJSON`/`apiresp.WriteError`, re-home the
   local error sentinels as aliases to the canonical `apiresp` set, and convert handler error
   writes to the canonical vocabulary (`unauthenticated`, `invalid_input`).
2. **Go full EntityResolver masking adoption** — extend partial resolver integration (1 of ~8
   external-UUID lookup sites) to full coverage, so a genuine entity miss masks to `403 forbidden`
   instead of leaking `404 not_found`.
3. **GUI error-handling migration** — replace hand-rolled `fetch`/error-parsing in
   `gui/src/lib/api.ts` with `@moduleforge/core-gui`'s shared typed `request()` wrapper, then
   migrate ad-hoc error-handling sites in `TagEditor.tsx`/`TagList.tsx` onto `useApiError` +
   `<FieldError>`/`<ErrorBanner>`/Toast.
4. **Documentation updates** — reconcile mod-tags' own docs (`AGENTS.md`, `next-steps.md`, etc.)
   with the new public error-envelope shape, canonical codes, and masking behaviour.

Explicitly out of scope: changes to other module repos, the design doc itself, wiring mod-tags
into any app's manifest, and standardizing the pre-existing `GET /entities/{uuid}/tags`
`{tags: […]}` vs. bare-array list-envelope asymmetry (flagged as a follow-up instead).

Phase 01 (Go) and Phase 02 (GUI) were structured as independent, parallelizable tracks (disjoint
file sets); Phase 03 (docs) ran last, after both landed. Because mod-tags is wired into no app
today, validation was limited to `go build`/`go test`, `bun run typecheck`/`build`, and the Ladle
workbench — there was no app-level regeneration step.

## What shipped

### Phase 01 — Go apiresp migration (`api/`)

- **001 — Response-layer migration** (merge `b9f7fd7`). Migrated mod-tags' `api/` layer onto the
  shared Wave-0 `apiresp` package: re-homed service sentinels as aliases, converted all handler
  success/error writes to `apiresp.WriteJSON`/`WriteError` with canonical codes
  (`unauthenticated`/`invalid_input`), and added an end-to-end test proving a masked
  `GET /tags/{uuid}` miss now returns 403 (nested envelope) — fixing the latent
  `entity.ErrForbidden`→500 bug. All validation (build, test, grep, manual scan) passed.
- **002 — EntityResolver masking adoption** (merge `35be9f8`). Extended masking coverage from the
  single already-migrated `GetByUUID` site to all remaining external-UUID lookups in `tag.go`:
  `Create` and `ListBySubject`'s subject-entity resolutions, `UpdateByUUID`/`UpdateTagValue`/
  `DeleteByUUID`'s pre-tx tag-entity resolutions (with residual in-tx `GetTagByEntityUUID` misses
  also masked to `ErrForbidden`), and `Search`'s owner/subject filter misses (initially converted
  to 403 instead of empty result — later reverted; see Key decisions). `ListBySubject`'s
  conversion directly resolved the open "verify list-route masking consistency" follow-up: the
  subject/parent entity is now resolved via `entityResolver.Resolve` before authorization, so an
  inaccessible subject masks to 403 exactly like a direct lookup. Updated six service tests and
  four handler tests, added two new service tests. All validation (build, vet, test, gofmt, grep)
  passed.
- **Post-merge gate fix** (merge `72002f9`, commit `2259c3e`): a phase-1 gate review found that
  the Search filter masking introduced by task 002 created a security bug — an entity-existence
  oracle. This was reverted; see Key decisions below.

### Phase 02 — GUI error handling (`gui/`)

- **001 — Adopt the shared `request()` client** (merge `857833a`). Rewrote `gui/src/lib/api.ts` to
  route all five `createTagsClient` methods through `@moduleforge/core-gui`'s `request()` wrapper,
  removing the bespoke `handleResponse`/`ErrorResponse`/throw-new-Error transport layer. Preserved
  `baseUrl` composition, `headers()` injection hook, purpose-filtering logic, JSON bodies, and
  `.tags` extraction. Re-exported (not redefined) `ApiError`/`ApiErrorResponse`/`FieldErrorData`/
  `ApiRequestError` from core-gui. Promoted `@moduleforge/core-gui` from an optional to a required
  peer dependency, adding an `overrides` entry so `bun install` resolves it via the local yalc
  store. Typecheck and build both green; public surface unchanged.
- **002 — Migrate ad-hoc error sites onto the widget set** (merge `06dab56`). Migrated all seven
  ad-hoc `err instanceof Error` catch sites across `TagEditor.tsx` (six) and `TagList.tsx` (one)
  onto core-gui's `useApiError` hook, rendering field-bound `invalid_input` details via
  `FieldError` and banner-level errors via `ErrorBanner`, with toast-worthy classes auto-dispatched
  to the Toast provider. Local string-based error state was replaced with typed
  `ApiRequestError | null` state; raw `role="alert"` markup removed entirely. Fixed a latent bug
  where `TagEditor`'s mutation-error banner could never display. `mockClient.ts` now throws
  representative `ApiRequestError`s; both story files gained new field/banner/toast stories wrapped
  in `ToastProvider`. Forwarded `ApiRequestError`/`ApiError`/`ApiErrorResponse`/`FieldErrorData`
  from `gui/src/index.ts`, closing a gap task 001 flagged. Typecheck, build, both required greps,
  and static + dev Ladle build/serve all passed.
- **Post-merge gate fix** (merge `ff629af`, commit `8323f72`): a phase-2 gate review found a
  security issue (unpinned peer dependency on the unclaimed `@moduleforge` npm scope) plus three
  correctness issues, all addressed; see Key decisions below.

### Phase 03 — Documentation updates

- **001 — Update architecture/module docs** (merge `1d8632c`). Reconciled mod-tags' documentation
  with the API-response behaviour changed by phases 01–02: corrected `next-steps.md`'s
  carry-forward integration scenario (item 9) from asserting 404 to the now-correct 403 for a
  genuine entity miss under masking, with a rationale note; added a short paragraph to
  `AGENTS.md`'s Router mounting section stating the canonical nested error envelope, error-code
  vocabulary, and masking-by-default (403 on genuine miss), linking to the design doc as the
  canonical source. `README.md` and `docs/decisions/tags-limited-immutability.md` required no
  changes. Validation grep confirmed no remaining stale error-status statements.
- **Post-merge gate fix** (merge `d5ef4ba`, commit `9bbdca6`): a phase-3 gate review found that the
  just-added `next-steps.md` item-9 language was itself inaccurate (claimed 403 where the code
  actually returns 404); corrected. See Key decisions below.

## Key decisions

The plan's phase-review gates caught three real bugs before they became permanent — one per
phase — each fixed in a dedicated post-merge commit on top of the phase's task merges.

- **Phase 1 gate: Search filter masking created an entity-existence oracle (security, fixed by
  revert).** Task 002 had converted `Search`'s owner/subject filter UUID misses from an empty
  result to a masked `403 forbidden`, mirroring the pattern used for direct lookups. The gate
  review found that `Search`'s only authorization gate is type-level and filter-independent
  (`Authorize(ctx, "list", &tagTypeID)`) — unlike `ListBySubject`, it has no instance-scoped
  authorization check tied to the specific filter entity. Combined with an
  existing-but-inaccessible filter UUID still returning `200 []`, the 403-on-miss conversion
  created a new entity-existence oracle spanning the entire entities table, reachable by any actor
  holding the broad "list tags" grant. Both Search filter sites were reverted to the pre-task-002
  "empty list on miss" behaviour — exactly the behaviour task 002's own requirement 6 text had
  pre-authorized as an acceptable fallback — with tests rewritten to match and the revert recorded
  in the task doc's Status section (commit `2259c3e`).
- **Phase 2 gate: unpinned peer dependency exposed a dependency-confusion risk (security, partially
  mitigated).** The `@moduleforge` npm scope is confirmed unclaimed on the public npm registry.
  Task 001 promoted `@moduleforge/core-gui` from an optional to a required peer dependency of
  `@moduleforge/tags-gui`. The gate review flagged this as a dependency-confusion / scope-squatting
  exposure. The fix commit (`8323f72`) applied a partial, code-level mitigation: pinning the peer
  dependency version and documenting the required local yalc-override step and the risk in the
  README. The gate also fixed three correctness issues in the same commit: documenting the hard
  `ToastProvider` dependency `useApiError` introduces on `TagEditor`/`TagList`; clarifying that
  core-gui's `authHandler` seam can override `headers()`'s `Authorization` value; and omitting
  `'purpose'` from `useApiError`'s fields option in fixed-purpose mode so an unexpected
  purpose-field error falls through to the banner instead of being silently dropped. Full
  resolution of the underlying exposure — reserving the `@moduleforge` npm scope (or an equivalent
  org-level fix) — is out of scope for a single plan and is tracked as a follow-up (see below).
- **Phase 3 gate: a documentation-accuracy bug in the just-landed doc fix (fixed).** Task 001's
  own doc correction to `next-steps.md` item 9 claimed the delete-then-get scenario now returns
  403. The gate review found this was itself inaccurate: the actual implemented behaviour returns
  404, because `GetByUUID`'s post-resolve tag-row fetch lacks existence-masking and still returns
  the un-migrated `ErrNotFound` — a known residual gap already tracked as follow-up `YM6y`. Commit
  `9bbdca6` corrected the doc to state that 403 applies only to UUIDs that never existed at all
  (the general masking case), while the delete-then-get case remains 404 pending that residual
  gap's resolution.

Other decisions of note, drawn from task digests:

- `ListBySubject`'s masking conversion (phase 01, task 002) was treated as directly resolving a
  pre-existing open follow-up ("verify list-route masking consistency") rather than merely
  incidental to the task's scope.
- The `GetByUUID`/site-1 post-resolve residual fetch was deliberately left unmigrated to 403 in
  phase 01 task 002 — it was explicitly out of that task's Requirements 1–6, per the masking
  audit's categorization of that site as "already masked... keep." This is the same gap the phase-3
  gate fix above had to account for.
- The list-envelope deviation (`GET /entities/{uuid}/tags` returning `{"tags": […]}` while
  `GET /tags` returns a bare array) was deliberately left intact as an explicitly out-of-scope,
  pre-existing asymmetry, per the overview's Known constraints section.

## Follow-up items

Carried forward in `plan/followups.yaml`:

- **Reserve the `@moduleforge` npm scope** (id `itQh`, phase-review: phase-02-gui-error-handling,
  security, major/medium confidence). The unclaimed public npm scope creates a dependency-confusion
  exposure shared by every `mod-*/gui` package that depends on another `@moduleforge/*` package,
  not just mod-tags. The phase-2 gate fix applied a partial code-level mitigation (pinned version +
  documented yalc-override convention), but full closure requires an org-level decision: reserve
  the scope on the public registry (even as placeholder packages), stand up a private registry, or
  formally document/enforce the yalc-override convention across all consumers. Surfaced for the
  project owner to decide.
- **Worktree `go.mod` replace-directive nesting mismatch** (id `rpwY`, go-apiresp-migration).
  `api/go.mod`'s committed replace directives assume a worktree sibling of `mod-core`, but task
  worktrees sit one level deeper, so `go build`/`go test` fail out of the box with "replacement
  directory does not exist." Worked around locally with a git-ignored worktree-root `go.work`;
  every task agent in a sibling mod-tags worktree hit this. Consider documenting/scripting the
  workaround centrally (e.g. in `create-worktree.sh` or the Go role doc).
- **Vestigial `httpapi.Deps.Logger`** (id `1NAf`, go-apiresp-migration). `*slog.Logger` in
  `api/httpapi/router.go` is unused by any non-test code and is now even more clearly vestigial
  since `apiresp.WriteError` does its own internal `slog` logging. Not touched — outside the
  originating task's diff.
- **`GetByUUID`'s residual post-resolve fetch still returns 404, not 403** (id `YM6y`,
  go-apiresp-migration). `tag.go:263`'s `GetTagByEntityUUID` call after the entity already resolved
  hits the same data-consistency edge case that sites 3–5 now mask to `ErrForbidden`, but was left
  unchanged (out of task 002's Requirements 1–6). Flagged as a minor residual inconsistency worth a
  follow-up decision. (This is the gap the phase-3 gate fix above had to document rather than
  resolve.)
- **Confirm Search's empty-list-on-miss semantics are the desired final behaviour** (id `nMxt`,
  go-apiresp-migration). Task 002's own doc flagged the (since-reverted) 403 conversion as a
  genuine behaviour change; now that it has been reverted back to empty-list, reviewers should
  confirm that is indeed the desired long-term semantics for search filters specifically, versus a
  future masked-403 approach paired with an instance-scoped Authorize check.
- **mod-core main is a moving target** (id `mHer`, go-apiresp-migration). An `apps/GetAppBySlug`
  interface addition landed mid-session on mod-core's main branch; will eventually require
  mod-tags' `mockCoreQuerier`/`fakeCoreQuerier` test mocks to be extended once mod-tags next syncs
  against mod-core's current main. Out of scope for this plan but worth tracking.
- **Redundant pre-tx `Resolve` call on the write hot path** (id `Xjc4`, phase-review:
  phase-01-go-apiserp-migration, efficiency, minor/high confidence). `UpdateByUUID`/
  `UpdateTagValue`/`DeleteByUUID` each gained a pre-tx `entityResolver.Resolve` call whose masking
  value is redundant on the success path, since the in-tx `GetTagByEntityUUID` fallback already
  maps `ErrNoRows` to `ErrForbidden`. Every successful call to these three write methods now costs
  one extra DB round trip. Deliberate, task-mandated pattern (mirrors `GetByUUID`'s precedent), but
  the trade-off was not explicitly weighed in the task doc. Consider whether the pre-tx call is
  still load-bearing, or explicitly document the accepted cost.
- **`gui/README.md`'s "Fresh checkout note" references a nonexistent `make link-tags` target** (id
  `jB0i`, gui-error-handling). Now that core-gui is a required peer dependency, a fresh checkout's
  `bun install` will hard-fail until a human runs the yalc-link step by hand, but no such Makefile
  target exists anywhere in the moduleforge tree. Recommend adding the referenced target or
  updating the README with the exact yalc `add`/`push` sequence. Developer-workflow gap, not
  currently CI-breaking (no CI exists for mod-tags today).
- **No test runner configured in `gui/`** (id `63jh`, gui-error-handling). No test script and no
  existing test files; the "errors are `ApiRequestError` instances" validation bullet for task 001
  was satisfied by code-path inspection rather than an executable test. Flagging in case test infra
  is wanted as a separate concern.
- **No automated component-test coverage for the new error-widget surfaces** (id `X1Ki`,
  gui-error-handling). No Testing Library/Vitest setup and no headless browser was available;
  validation for task 002 rested on static build, dev-server story discovery, and manual code
  review. A reasonable follow-up would add Testing Library plus a `ToastProvider`-wrapped render
  harness.
- **`gui/build/` is tracked in git despite being a generated artifact** (id `PXOK`,
  gui-error-handling). Predates this plan (added in a single prior commit). Running `ladle build`
  for validation regenerated it with new content-hashed filenames, which was reverted to avoid an
  unrelated diff. Worth a follow-up decision on whether the directory should be gitignored instead
  of committed.
- **Task doc's own `## Status` subsection not updated for phase-02 task 002** (id `IbKQ`,
  gui-error-handling). The task agent reasoned (incorrectly, on reflection) that the task doc lived
  outside its worktree boundary. No functional impact — the authoritative record is `TODO.yaml` via
  the `apply_report` call, not the task doc's optional Status section.

Also noted in the plan overview but not in `followups.yaml`: the pre-existing list-envelope
asymmetry between `GET /entities/{uuid}/tags` (`{"tags": […]}`) and `GET /tags` (bare array) was
deliberately left out of scope and is recommended as a future standardization follow-up.

## Final Task State

# TODO

## Purpose and scope

Tracking document for the active plan.

## Tasks

### Phase 01 — Go Apiresp Migration

- [x] [001-response-layer-migration.md](./phase-01-go-apiresp-migration/001-response-layer-migration.md) — tier `sonnet-high` · branch `phase-01-task-01-response-layer-migration` · commit `9ecc67d` · merge `b9f7fd7362e8788e02349e2e115a7431389af0cb`
- [x] [002-entityresolver-masking-adoption.md](./phase-01-go-apiresp-migration/002-entityresolver-masking-adoption.md) — tier `sonnet-high` · branch `phase-01-task-02-entityresolver-masking-adoptio` · commit `d875cce` · merge `35be9f8c0bb9ae4e06eeedf42ead591b23f3fa7f`

### Phase 02 — GUI Error Handling

- [x] [001-adopt-shared-request-client.md](./phase-02-gui-error-handling/001-adopt-shared-request-client.md) — tier `sonnet-high` · branch `phase-02-task-01-adopt-shared-request-client` · commit `15e94e4` · merge `857833a228e7406b72b133e6940557a8f983e7f1`
- [x] [002-migrate-error-widgets.md](./phase-02-gui-error-handling/002-migrate-error-widgets.md) — tier `sonnet-high` · branch `phase-02-task-02-migrate-error-widgets` · commit `ad05d99` · merge `06dab5661fb0c0c165b50837811d90e03c39ef41`

### Phase 03 — Documentation Updates

- [x] [001-update-architecture-docs.md](./phase-03-doc-updates/001-update-architecture-docs.md) — tier `sonnet-high` · branch `phase-03-task-01-update-architecture-docs` · commit `0d78367` · merge `1d8632cc4e0b1ae772bf62186e1e377abf711486`
