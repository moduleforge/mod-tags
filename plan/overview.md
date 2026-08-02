# One-Of-Domain Tag Purpose Exclusivity

## Purpose and scope

Add a per-`purpose` boolean, `one_of_domain`, that governs whether an entity may hold
more than one tag of a given `purpose` at a time, enforced (to the extent practical) at
the DB, API, and GUI layers of `mod-tags`.

**Resolved terminology.** The user's original phrase "domain" was clarified directly
with the user to mean the existing `purpose` column on the `tags` table
(`model/migrations/0201_tags.sql`) — there is no separate "domain" table/column
introduced by this plan.

**Current behavior (being changed).** `tags` currently carries an unconditional
`UNIQUE INDEX tags_owner_subject_purpose_idx ON tags (owner_id, subject_id, purpose)` —
for every purpose, an owner can never attach two tags of the same purpose to the same
subject. This plan makes that a *conditional* rule, gated per purpose by the new
`one_of_domain` flag:

- `one_of_domain = true` for a purpose → today's behavior is retained: at most one tag
  of that purpose per (owner, subject).
- `one_of_domain = false` for a purpose → multiple tags of that purpose are allowed on
  the same subject (from the same owner).
- **Default for purposes with no policy row (ad hoc/unregistered purposes):
  `one_of_domain = false`** (multiple allowed) — matches the user's explicit
  recommendation and today's desired end-state default.

This document also carries the open design-question resolutions the user asked to have
recorded here explicitly (not silently assumed) — see "Design decisions" below.

## Current status

No work has started. `model/migrations/` currently ends at `0204_tag_templates.sql`
(plus the always-highest `0299_access_function_stubs.sql` override slot); `tags`' only
enforcement of purpose uniqueness today is the unconditional unique index above.
Phase 1 (`model-one-of-domain`) begins first; it has no external preconditions beyond
the current `main` branch state of this repo.

**Deviation from the standard planning procedure, flagged for the manager:** this plan
registers all four phases below with complete task breakdowns and returns
`status: complete` in a single invocation, rather than following the
`analyze-change-request` skill's Phase 3B multi-phase mechanism verbatim (which always
returns `needs input` with research/user-question lists). Investigation for this
request turned up no genuine knowledge gaps or user-only decisions beyond what the
user's own request already authorized the planner to resolve and record (see "Design
decisions" below) — returning `needs input` with empty `research_requests`/
`user_questions` lists would only add a needless round trip. See this task's structured
report for full reasoning.

## Design decisions

The user's request enumerated five open design questions and asked that they be
resolved (or explicitly flagged) here, with rationale, rather than silently assumed.

### 1. Where does `one_of_domain` live?

**Decision: a new table, `tag_purpose_policies (purpose TEXT PRIMARY KEY, one_of_domain
BOOLEAN NOT NULL DEFAULT false, created_at, updated_at)`** — keyed purely on `purpose`,
no `scope` column.

**Why not `tag_templates`?** `tag_templates` is keyed per `(scope, purpose, value)` — a
finer grain than `purpose` alone — and is an explicitly **open, read-only, non-
authorization-gated** suggestion catalog for UI pickers (see `model/README.md`,
`README.md`'s "Core features"). Putting an enforcement-relevant flag there would
require either (a) forcing every row sharing a `purpose` to agree on the flag (new
constraint machinery, and an unclear answer for whether a `scope`-scoped row could
disagree with a global row for the same purpose), or (b) a value-less placeholder row,
which doesn't fit `tag_templates`' `value NOT NULL` + `(scope, purpose, value)` unique
design. A separate table keyed only on `purpose` avoids both problems cleanly and keeps
the read-only suggestion catalog semantically pure.

**No `scope` column.** The request frames `one_of_domain` purely per-`purpose`, and
purpose semantics (e.g. "priority", "status") are conceptually global, not per-app.
This is a deliberate scoping decision, not an oversight — see `docs/project-roadmap.md`
for how a future scoped variant would be a separate, not-yet-designed extension.

**Naming note.** `docs/project-roadmap.md` already anticipates an unrelated, deferred
`tag_qualifier_policies` table (open/closed *value* catalog enforcement — whether
arbitrary values are accepted for a purpose at all). `tag_purpose_policies` (this plan)
is conceptually distinct — it governs *how many* tags of a purpose may coexist, not
*which values* are allowed. The roadmap doc is updated to call out this distinction
explicitly so the two are not conflated later.

### 2. Default for unregistered purposes

**`one_of_domain = false`** (multiple tags of that purpose allowed) when no
`tag_purpose_policies` row exists for a purpose — implemented as the trigger's
`NOT FOUND` fallback (see Decision 3). Matches the user's explicit recommendation.

### 3. DB enforcement mechanism

A plain (partial) unique index cannot express "conditionally unique based on a value
looked up in another table." Enforcement moves to a `BEFORE INSERT` trigger on `tags`,
mirroring this module's existing `tags_check_type()` / `tags_reject_immutable_changes()`
trigger style (`model/migrations/0201_tags.sql`):

- The existing unconditional `UNIQUE INDEX tags_owner_subject_purpose_idx` is dropped
  and replaced by a **plain (non-unique) index of the same name and columns** — the
  index itself remains, only its uniqueness is removed, so lookup performance for the
  trigger's own check (and any other `(owner_id, subject_id, purpose)` query) is
  unaffected.
- A new trigger function, `tags_enforce_one_of_domain()`, looks up `one_of_domain` for
  `NEW.purpose` (defaulting to `false` when absent — Decision 2) and, only when `true`,
  rejects the insert if a tag already exists for `(owner_id, subject_id, purpose)`.
- **`BEFORE INSERT` only — not `UPDATE`.** `purpose` (and `owner_id`/`subject_id`) are
  already immutable after insert via `tags_reject_immutable_changes`, so no `UPDATE`
  path can create a new one-of-domain conflict; only `INSERT` needs the check.
- **Race-safety.** A trigger-based `EXISTS` check has a TOCTOU gap a true unique index
  does not: two concurrent inserts for the same `(owner_id, subject_id, purpose)` could
  each pass the check before either commits. The trigger closes this by taking a
  transaction-scoped advisory lock (`pg_advisory_xact_lock`, keyed on a hash of the
  tuple) before the `EXISTS` check, serializing concurrent inserts for the same tuple.
  This is a deliberate, documented mitigation — flagged here as a known characteristic
  of the trigger-based approach relative to a true unique index (accepted because a true
  conditional unique index is not expressible in Postgres).
- **SQLSTATE reuse.** The trigger raises its exception `USING ERRCODE = 'unique_violation'`
  (the standard `23505` code) — the *same* SQLSTATE the previous unconditional unique
  index violation, deliberately, so `api/service/tag.go`'s existing
  `pgErr.Code == pgUniqueViolation` → `ErrConflict` classification in `TagService.Create`
  requires **no Go code changes** to correctly map a one-of-domain conflict to `409
  conflict`.
- Trigger name `tags_enforce_one_of_domain` sorts alphabetically after
  `tags_check_type` (so entity-type validation still runs first) and before
  `tags_reject_immutable_changes`/`tags_set_updated_at` — consistent with this module's
  existing "name chosen to sort alphabetically" convention.

New migration: `model/migrations/0205_tags_one_of_domain.sql` (next available number
after `0204`; `0299` remains the reserved, always-last access-function override slot —
see `moduleforge.module.yaml`'s `migrations.range` and `model/README.md`).

### 4. API surface

- **Conflict surfacing.** No Go code changes are needed for the error *classification*
  itself (see Decision 3's SQLSTATE reuse) — Phase 2 adds a test proving it end-to-end.
- **Exposing "occupied" purposes to the GUI.** Every tag-returning response (`Tag`
  service type, and therefore `POST /tags`, `GET /tags/{uuid}`, `GET /tags`,
  `GET /entities/{uuid}/tags`, `PUT /tags/{uuid}`, `PATCH /tags/{uuid}`) gains a new
  `oneOfDomain: boolean` field per tag, computed via a join/subquery against
  `tag_purpose_policies` in the underlying `sqlc` queries. **Chosen over a new lookup
  endpoint**: "occupied" is only meaningful for a purpose that already has an existing
  tag on the subject — exactly what `GET /entities/{uuid}/tags` (which the GUI already
  calls on every `TagEditor`/`TagList` mount) enumerates. Embedding the flag avoids a
  second round trip and a new endpoint, and applying it uniformly across all
  tag-returning endpoints (not just the list one) keeps the wire `Tag` shape consistent
  module-wide.
- **Flag CRUD home.** Mirrors `tag_templates`' existing convention exactly:
  `TagTemplateServicer.Upsert` is documented as "an internal/administrative capability
  meant for in-process callers ... not an HTTP-exposed operation; no route calls it."
  `TagPurposePolicyServicer` (new, Phase 2) follows the same shape — an internal-only
  `Upsert`/`Get`, **no HTTP route registered for it in this plan**. This is a deliberate
  choice, not an oversight: `one_of_domain` is a curated, admin-set value, not something
  this plan exposes for end-user or public API writes. Values are seeded/managed
  out-of-band (direct SQL, a future admin surface, or a consuming app's own startup
  hook) until a curated admin surface is designed — flagged explicitly in
  `next-steps.md`.

### 5. GUI requirements

**"Combo-search picker" interpretation.** `TagEditor` (`gui/src/TagEditor.tsx`) today
has **no dedicated combo-search/autocomplete widget** and no wiring to the
`tag_templates` catalog at all (confirmed by inspection — there is no fetch of
`/tag-templates` anywhere in `gui/src/`). Its purpose-entry UI is one of: a free-form
`<input>`, a fixed `<span>` (single-purpose mode), or a plain `<select>` populated from
the host-app-supplied `purposes` prop (multi-purpose mode). This plan treats that
existing UI as "the picker" the request refers to and does **not** introduce a new
combo-box/autocomplete widget wired to `tag_templates` — that would be substantial,
unrelated net-new scope. **This is an assumption applied, flagged for manager
visibility**; see the structured report.

**Root cause of the reported bug** (adding `priority:low` then `priority:urgent` was
incorrectly allowed client-side). `TagEditor.handleAddSubmit` performs no client-side
purpose-conflict check today at all — it validates only that `purpose`/`value` are
non-empty and submits directly; any duplicate-purpose rejection today depends entirely
on the server round trip (which itself may or may not reject depending on whether the
two tags share an owner). **Resolution: fixed as part of this plan's GUI phase**
(the user's recommended option), since the new `one_of_domain`-aware exclusion logic
requires building this exact client-side check anyway.

**New client-side logic** (Phase 3, `gui/src/TagEditor.tsx`):
- Compute `occupiedPurposes` = the set of purposes among the subject's already-loaded
  `tags` where `oneOfDomain === true`.
- `<select>` (multi-purpose) mode: exclude any purpose in `occupiedPurposes` from the
  rendered `<option>` list.
- All modes (`select`, free-form, fixed): before calling `client.create`, if the
  purpose to submit is in `occupiedPurposes`, block submission client-side with a
  synthesized field error (mirrors the existing required-field pre-submit-validation
  pattern) instead of relying on the server round trip.

## Consuming-app note (flagged, not in scope here)

`TagEditorProps`/`Tag`'s public shape gains only an additive `oneOfDomain` field — no
existing prop or method signature changes, so `mod-users`' existing
`<TagEditor subject=... client=... />` usage (via yalc-linked `tags-gui`) continues to
work unchanged. The only consumer follow-up is the routine one of refreshing the
yalc-linked `tags-gui`/`tags-api` snapshot to pick up the new field/behavior once this
plan lands — not unique to this feature, so it is flagged for the manager rather than
made a task here, consistent with mod-tags being planned/executed independently of its
consumers. (See also pre-existing, separately-tracked followup `QDH5` on
`mod-tags/gui/package.json`'s `core-gui` hard-dependency issue, which is unrelated to
this feature but affects real `bun install`-based GUI validation in consuming-app task
worktrees; not addressed here.)

## Overview

Four phases, strictly sequential (data model → API → GUI → docs), matching this
change's natural dependency order: the GUI task needs the API's `oneOfDomain` wire
field to exist; the API task needs the model's schema/queries to exist.

### Phase 1 — `model-one-of-domain`: schema and queries

1. `001-add-tag-purpose-policies-table.md` — new migration
   (`0205_tags_one_of_domain.sql`) adding `tag_purpose_policies`, converting
   `tags_owner_subject_purpose_idx` to non-unique, and adding the
   `tags_enforce_one_of_domain` trigger; new `model/queries/tag_purpose_policies.sql`
   queries; `sqlc generate`.
2. `002-expose-one-of-domain-in-tag-queries.md` — extend the six existing tag
   read/write queries (`CreateTag`, `GetTagByEntityUUID`, `ListTagsBySubjectEntityID`,
   `SearchTags`, `UpdateTagColor`, `UpdateTagValue`) in `model/queries/tags.sql` to
   return a computed `one_of_domain` column; `sqlc generate`. **Depends on task 001**
   (the `tag_purpose_policies` table must exist first).

### Phase 2 — `api-one-of-domain`: service and HTTP responses

1. `001-add-tag-purpose-policy-service.md` — new `TagPurposePolicyServicer`
   (`api/service/tag_purpose_policy.go`), internal-only, no HTTP route, mirroring
   `TagTemplateServicer.Upsert`'s convention. **Parallel-eligible** with Phase 2 task
   002 (touches different files).
2. `002-thread-one-of-domain-through-tag-api.md` — thread the new `oneOfDomain` field
   through `service.Tag`, all `hydrateTag*` helpers, `TagServicer` call sites, and
   `httpapi`'s `tagResponse`/`toTagResponse`; update `mockTagQuerier`/`mockCoreQuerier`
   test doubles; unit test proving a simulated one-of-domain conflict classifies as
   `ErrConflict`; integration test proving the real DB trigger does too.

Both Phase 2 tasks depend on Phase 1 being complete (they import the regenerated
`tagsdb` package).

### Phase 3 — `gui-one-of-domain`: picker exclusivity and pre-submit validation

1. `001-tag-editor-one-of-domain-filtering.md` — `gui/src/lib/api.ts`'s `Tag` gains
   `oneOfDomain: boolean`; `TagEditor.tsx` gains occupied-purpose `<select>` filtering
   and pre-submit exclusivity validation (fixing the reported bug as part of the same
   change); `mockClient.ts` and `TagEditor.stories.tsx` updated/extended.

Depends on Phase 2 (needs the finalized `oneOfDomain` wire field).

### Phase 4 — `docs-one-of-domain`: documentation

1. `001-document-one-of-domain.md` — `README.md` "Core features" bullet;
   new `docs/decisions/tags-one-of-domain.md` (mirrors
   `docs/decisions/tags-limited-immutability.md`'s format); `next-steps.md` manual
   verification + carry-forward items; `docs/project-roadmap.md` note distinguishing
   this feature from the deferred `tag_qualifier_policies` idea and flagging a possible
   future broader "domain"-above-`purpose` grouping extension.

Depends on Phases 1–3 (documents the landed field names/behavior accurately).

**No generic `doc-updates` phase.** This repository has no `docs/architecture.md` or
`docs/*-spec.md` file (`docs/mf-standards/` is a read-only git submodule, not owned by
this repo) — the `analyze-change-request` skill's Phase 4 architectural-implications
mechanism targets exactly those files, which don't exist here. Phase 4 above (driven
directly by the user's own request) covers the documentation this change needs instead;
see the structured report for this explicitly-flagged deviation.
