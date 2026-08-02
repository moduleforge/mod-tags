# Plan Summary — `tags-one-of-domain`

## What was planned and why

The user reported a bug: adding a `priority:low` tag and then a `priority:urgent` tag
to the same subject was incorrectly allowed client-side, when purposes like `priority`
are meant to be mutually exclusive per subject. The request asked for a per-`purpose`
boolean — resolved in scope to mean the existing `purpose` column on `tags`
(`model/migrations/0201_tags.sql`), not a new "domain" table/column — that governs
whether an entity may hold more than one tag of a given purpose at a time, enforced
(to the practical extent possible) at the DB, API, and GUI layers of `mod-tags`.

Today `tags` carries an unconditional
`UNIQUE INDEX tags_owner_subject_purpose_idx ON tags (owner_id, subject_id, purpose)`.
This plan turns that into a *conditional* rule gated by a new `one_of_domain` flag:
`true` retains today's at-most-one-tag-per-purpose behavior; `false` allows multiple
tags of that purpose on the same subject; and, per the user's explicit recommendation,
purposes with **no** policy row default to `one_of_domain = false` (multiple allowed).

The plan resolved five open design questions directly in `plan/overview.md` (policy
table placement, unregistered-purpose default, DB enforcement mechanism, API surface,
and GUI requirements) rather than leaving them to be silently assumed, and then
executed four strictly sequential phases (data model → API → GUI → docs) matching the
natural dependency order: the GUI phase needs the API's `oneOfDomain` wire field, and
the API phase needs the model's schema/queries.

## What shipped

All 4 phases / 6 tasks completed and merged to the plan branch.

### Phase 1 — `model-one-of-domain`: schema and queries

- **Task 001 — Add Tag Purpose Policies Table And Enforcement Trigger**
  (merge `9808408ca60d76f721c642454011af7cd1f10664`). Implemented the new
  `tag_purpose_policies` table plus its `updated_at` trigger, converted
  `tags_owner_subject_purpose_idx` to a non-unique index, and added the
  `tags_enforce_one_of_domain` `BEFORE INSERT` trigger, all in new migration
  `model/migrations/0205_tags_one_of_domain.sql`. Added
  `GetTagPurposePolicy`/`UpsertTagPurposePolicy` queries and regenerated
  `model/db` (`tags.sql.go` untouched at this point). All validation (goose
  validate, sqlc compile, ephemeral-Postgres shadow-DB apply, greps, README
  check) passed.
- **Task 002 — Expose One Of Domain In Tag Queries**
  (merge `6201d9071fe69c5b99597a5cc829dce8a5aa4e35`). Extended all six named
  tag queries in `model/queries/tags.sql` (`CreateTag`, `UpdateTagColor`,
  `UpdateTagValue` via a scalar `COALESCE` subquery on `RETURNING`;
  `GetTagByEntityUUID`, `ListTagsBySubjectEntityID`, `SearchTags` via a
  `LEFT JOIN tag_purpose_policies` + `COALESCE(tpp.one_of_domain, false)`) to
  surface a computed `one_of_domain` boolean, defaulting to `false` when no
  policy row exists. `GetTagByEntityID` was deliberately left untouched.
  Regenerated sqlc; `CreateTag`/`UpdateTagColor`/`UpdateTagValue` now return
  new `*Row` types instead of a bare `Tag`, and the three `Get*`/`List*`/
  `Search*` row types simply grew an `OneOfDomain bool` field. This was
  documented as intentionally, deliberately breaking `api/`'s build
  (three known call sites in `api/service/tag.go`) as input for Phase 2.

### Phase 2 — `api-one-of-domain`: service and HTTP responses

- **Task 001 — Add Tag Purpose Policy Service** (merge
  `145a5cc428fd5e3d2ad5d4dcf6651ebc9bc08012`). No structured-report digest was
  recorded for this task; its outcome is drawn from the task doc's own
  `## Status` section instead. New `TagPurposePolicyServicer`/
  `TagPurposePolicyService` (`api/service/tag_purpose_policy.go`), mirroring
  `TagTemplateServicer`'s shape: internal-only `Get`/`Upsert`, no HTTP route,
  wired into the `Services` aggregate. `Get` on an unset purpose returns
  `{OneOfDomain: false}` with no error, matching the DB trigger's own default.
  Whole-package `go build`/`go test`/`make lint` reported "failed" at the time
  of this task's own validation, but for reasons entirely pre-existing and
  out of this task's scope — the three `hydrateTag` call sites Phase 1 task
  002 deliberately left broken as input for this phase's task 002. The task's
  own additions were independently verified correct via a local, uncommitted
  patch that unblocked the package (all new tests passing 6/6); this task's
  own bullets (service.go wiring grep, no-HTTP-route grep) passed cleanly.
- **Task 002 — Thread One Of Domain Through Tag Api** (merge
  `9928d72e3bb791892c9346f272bb30bcce708b77`). Threaded the new
  `one_of_domain` boolean end-to-end: added `Tag.OneOfDomain`, changed
  `hydrateTag`'s signature to take an explicit `oneOfDomain bool` parameter
  (fixing the three pre-existing `CreateTagRow`/`UpdateTagColorRow`/
  `UpdateTagValueRow` build errors task 001 had flagged as expected/deferred),
  updated all six call sites, deliberately left `tagSnapshot` untouched, and
  added `oneOfDomain` to the httpapi `tagResponse`/`toTagResponse`. Updated
  `mockTagQuerier`/`singleTagQuerier` test doubles to satisfy the regenerated
  `tagsdb.Querier` interface, reading `OneOfDomain` from task 001's shared
  policies map. Added mock-backed unit tests proving the conflict is
  purpose-conditional, and a new `//go:build integration` test proving the
  real Postgres trigger raises the same SQLSTATE and `TagService.Create`'s
  existing classification logic catches it with zero Go changes. The whole
  `api` package (build, vet, unit tests, lint, integration suite) was green
  at HEAD by the end of this task.

### Phase 3 — `gui-one-of-domain`: picker exclusivity and pre-submit validation

- **Task 001 — Tag Editor One Of Domain Filtering** (merge
  `a302936bc9ced7efa90d4d44bcfe87f04bbf0755`). Implemented client-side
  one-of-domain exclusivity in `TagEditor`: added the required `oneOfDomain`
  wire field to `Tag`, derived an `occupiedPurposes` set from the
  already-loaded tag list, filtered it out of the multi-purpose select's
  options, and added a uniform pre-submit guard in `handleAddSubmit` that
  blocks any mode from submitting an occupied purpose — fixing the
  originally reported bug. Surfaced the resulting field error in
  fixed-purpose mode (previously had no `FieldError` slot). Updated the mock
  client to default `oneOfDomain: false` and added a
  `SelectPurposeOneOfDomain` Ladle story. Two `Tag`-fixture builders outside
  the task doc's named file list (`TagChip.stories.tsx`,
  `TagList.stories.tsx`) needed the same one-line default to keep
  type-checking — applied as an in-scope, mechanically-necessary same-diff
  fix. Full `tsc --noEmit` validation remained blocked by a pre-existing,
  unresolved `@moduleforge/core-gui` module-resolution issue distinct from
  what followup `QDH5` originally diagnosed (package.json already correctly
  uses the optional-peer pattern; `bun install` succeeds; the remaining
  blocker is no local `core-gui` build being reachable).

### Phase 4 — `docs-one-of-domain`: documentation

- **Task 001 — Document One Of Domain Feature** (merge
  `b61b11b2169d9718299cb79b9d3dc55f3d04dbc3`). Documented the feature landed
  by Phases 1–3: a new "Core features" bullet and "Additional documentation"
  link in `README.md`; a new decision record
  `docs/decisions/tags-one-of-domain.md` mirroring
  `tags-limited-immutability.md`'s structure, covering the
  `tag_purpose_policies` table shape, the false-when-absent default, the
  trigger-based enforcement mechanism (full SQL listing, advisory-lock race
  mitigation, and the SQLSTATE-reuse trick), the API surface (embedded
  `oneOfDomain` field, no public write endpoint), and the GUI decisions; two
  new bullets in `next-steps.md` (manual-verification smoke test and
  carry-forward items); and a new roadmap bullet distinguishing
  `one_of_domain`/`tag_purpose_policies` from the unrelated
  `tag_qualifier_policies` idea. Content was cross-checked against the
  actual landed migration, service, HTTP, and GUI code, and this pass is
  where the trigger-firing-order rationale error (see below) was finally
  corrected in the decision record's own text.

## Key decisions

- **Table placement: new `tag_purpose_policies`, not `tag_templates`.**
  `tag_templates` is keyed per `(scope, purpose, value)` — finer-grained than
  `purpose` alone — and is an explicitly open, read-only, non-authorization-gated
  suggestion catalog. Putting an enforcement-relevant flag there would require
  either forcing every row sharing a `purpose` to agree, or a value-less
  placeholder row that doesn't fit `tag_templates`' `value NOT NULL` unique
  design. A separate table keyed only on `purpose`, with no `scope` column
  (purpose semantics are conceptually global, not per-app), avoids both
  problems and keeps the read-only catalog semantically pure.
- **Default `one_of_domain = false` for unregistered purposes.** Implemented
  as the trigger's `NOT FOUND` fallback and mirrored in the service layer's
  `Get`, matching the user's explicit recommendation.
- **Trigger-based enforcement with advisory-lock race mitigation and
  SQLSTATE reuse.** A plain (partial) unique index cannot express
  "conditionally unique based on a value looked up in another table," so
  enforcement moved to a `BEFORE INSERT` trigger,
  `tags_enforce_one_of_domain`, mirroring this module's existing trigger
  style. The previously-unconditional unique index was converted to a plain
  index of the same name/columns (index retained for lookup performance,
  only uniqueness removed). Because a trigger-based `EXISTS` check has a
  TOCTOU gap a true unique index doesn't, the trigger takes a
  transaction-scoped `pg_advisory_xact_lock` (keyed on a hash of the tuple)
  before the check, serializing concurrent inserts for the same
  `(owner_id, subject_id, purpose)` — a deliberate, documented mitigation,
  accepted because a true conditional unique index isn't expressible in
  Postgres. The trigger raises its exception `USING ERRCODE =
  'unique_violation'` (the same `23505` SQLSTATE the old unique index used),
  so `api/service/tag.go`'s existing `pgErr.Code == pgUniqueViolation` →
  `ErrConflict` classification required **zero** Go code changes.
- **No HTTP route for the policy servicer.** `TagPurposePolicyServicer`
  mirrors `TagTemplateServicer.Upsert`'s existing "internal/administrative,
  not HTTP-exposed" convention exactly. `one_of_domain` is a curated,
  admin-set value seeded/managed out-of-band until a curated admin surface
  is designed — a deliberate scope boundary, not an oversight.
- **"Picker" = the existing `TagEditor` input, not a new combo-box.**
  `TagEditor` had no dedicated combo-search/autocomplete widget and no
  wiring to the `tag_templates` catalog at all prior to this plan. The plan
  treated its existing free-form `<input>` / fixed `<span>` / `<select>` UI
  as "the picker" the original request referred to, rather than introducing
  new, substantially out-of-scope combo-box machinery — flagged explicitly
  for manager visibility in `plan/overview.md`.
- **`oneOfDomain` embedded on every tag-returning response, no new
  endpoint.** "Occupied" is only meaningful for a purpose that already has
  an existing tag on the subject — exactly what `GET /entities/{uuid}/tags`
  already enumerates on every `TagEditor` mount. Embedding the flag
  uniformly across all tag-returning endpoints avoids a second round trip
  and a new endpoint, and keeps the wire `Tag` shape consistent module-wide.

### Notable thread: a recurring, self-corrected documentation error

The alphabetical-trigger-ordering rationale (used to justify why
`tags_enforce_one_of_domain` should fire after entity-type validation) named
a trigger, `tags_check_type`, that doesn't actually exist under that name in
the codebase — the real `BEFORE INSERT` trigger in
`model/migrations/0201_tags.sql` is named `tags_type_check`
(`tags_check_type` is only the trigger *function* name). Comparing the real
trigger names, `tags_enforce_one_of_domain` ('e') actually sorts *before*
`tags_type_check` ('t'), the reverse of the original claim — harmless in
practice, since the one-of-domain check only reads `NEW`'s own columns and
doesn't depend on the type-check trigger's effects, but a real inversion of
the stated design rationale. This surfaced three separate times across the
plan (flagged as an ambiguity in the Phase 1 review, again in the Phase 2
review, and finally caught baked into Phase 4's own decision-record draft),
where it was ultimately corrected in `docs/decisions/tags-one-of-domain.md`
(see "The BEFORE INSERT trigger that validates entity type is named
`tags_type_check`..." in that file). Tracked as followup `xb4v`. No data-
integrity impact — only affects which error surfaces first / an extra
advisory-lock acquisition on an insert that's going to fail anyway.

## Follow-up items

Selected items from `plan/followups.yaml` specific to this plan (tag
`plan/phase:model-one-of-domain`, `api-one-of-domain`, or
`gui-one-of-domain`) — see that file for the full text and for older,
pre-existing items carried from prior plans:

- **`xb4v`** — the trigger-firing-order documentation error described above;
  manager should decide whether any further correction (e.g. renaming a
  trigger) is warranted beyond the decision-record fix already made.
- **`wp4b`** — confirms the three `hydrateTag` call-site build breaks caused
  by Phase 1 task 002's sqlc regeneration were expected/deferred input for
  Phase 2 task 002; no action needed (already resolved by that task).
- **`NcgX`** — the `::boolean` cast in the shipped migration SQL is a minor
  textual deviation from the task doc's literal example SQL; generated Go
  types and runtime semantics match intent exactly. Flagged in case the
  planner wants to update the task doc's example for future reference.
- **`lzuZ`** — `model/migrations/0205_tags_one_of_domain.sql`'s
  `DROP INDEX` + non-concurrent `CREATE INDEX` rebuild of
  `tags_owner_subject_purpose_idx` runs inside the migration's transaction
  against the already-populated `tags` table, taking a brief
  `ACCESS EXCLUSIVE` lock and blocking writes for the rebuild's duration —
  a one-time deploy-time cost, likely a non-issue while `tags` stays small,
  but worth a judgment call (or a future `NO TRANSACTION` + `CONCURRENTLY`
  split migration) if a target environment's `tags` table is non-trivial
  before 0205 lands.
- **`VTKH`** — the `pg_advisory_xact_lock` acquired by
  `tags_enforce_one_of_domain` is transaction-scoped, so for a
  one-of-domain purpose it stays held until `TagService.Create`'s enclosing
  transaction commits — including the trailing observer/audit write after
  the insert — slightly widening serialization for concurrent inserts on
  the same tuple. No correctness impact today; revisit if the observer path
  grows slower or gains external calls.
- **`wmwp`** — the sandbox's local Homebrew `postgresql@14` service
  permanently occupies `127.0.0.1:5432`/`[::1]:5432`, shadowing the
  Docker-based test Postgres container for both of `resolvePostgresHost`'s
  existing candidates; worked around via an env var for this session, but
  likely to recur for any local dev machine running native Postgres
  alongside Docker. Possible follow-up: have `resolvePostgresHost` probe a
  third, LAN-IP-style candidate automatically.
- **`Lesl`** — `TagPurposePolicyService.Get`/`Upsert` has no upper-bound
  length check on `purpose` (unlike `TagService.Create`'s 512-char cap). Not
  currently reachable from any HTTP route, so no current exploit surface;
  matters only if a future admin surface wires an HTTP handler to this
  service without its own check.
- **`TdYQ`** — the "add a required field" ripple effect on
  `TagChip.stories.tsx`/`TagList.stories.tsx` fixture builders wasn't
  anticipated by the task doc's named-file Requirements list; worth noting
  for future task-doc scoping on similar changes.
- **`kkkC`** — `bun install` no longer trips on followup `QDH5`'s originally
  diagnosed cause in this worktree (package.json already uses the
  optional-peer pattern), but full `tsc --noEmit`/Ladle validation is still
  blocked because no `@moduleforge/core-gui` package/build is reachable on
  this machine. Possibly `QDH5` is already resolved on `main` and the
  remaining blocker is a distinct "no local `core-gui` build to link"
  problem — worth re-examining under `QDH5` or as a separate followup.
- **`INO7`** — `TagEditor.tsx`'s fixed-purpose branch added
  `aria-describedby` on a plain, non-interactive `<span>`, mirroring the
  pattern used by the free-form `<input>`/`<select>`; the accessible
  association's screen-reader effectiveness is uncertain for a non-focusable
  element. Worth confirming with a screen reader, or adjusting the wiring.
- Also relevant background (pre-existing, not new to this plan):
  **`QDH5`** (`core-gui` hard-dependency breaking `bun install` in
  consuming apps) is referenced directly in `plan/overview.md`'s
  "Consuming-app note" as affecting real GUI validation in this plan's
  Phase 3 task, and its status is further clarified by `kkkC` above.

## Consuming-app note

Per `plan/overview.md`: `TagEditorProps`/`Tag`'s public shape gained only an
additive `oneOfDomain` field — no existing prop or method signature changed,
so `mod-users`' existing `<TagEditor subject=... client=... />` usage (via
yalc-linked `tags-gui`) continues to work unchanged. The only consumer
follow-up is the routine refresh of the yalc-linked `tags-gui`/`tags-api`
snapshot to pick up the new field/behavior — flagged for the manager rather
than made a task here, consistent with `mod-tags` being planned/executed
independently of its consumers.
