# Document One Of Domain Feature

## Purpose and scope

Document the `one_of_domain` feature landed by Phases 1–3: update `README.md`'s "Core
features" list, add a new decision record, and update `next-steps.md` /
`docs/project-roadmap.md`. **Depends on** Phases 1–3 being complete (documents the
actual landed field names/behavior, not a preliminary design). This task does not
touch `model/README.md` (already updated in Phase 1 task 001) or `api/README.md`/
`gui/README.md` (pre-existing stale placeholders, already flagged out of scope by
followups `QRHG`/similar — do not touch them here either).

## Requirements

### 1. `README.md` — "Core features" bullet

Add one bullet to the existing list (after "Tag-templates catalog"), e.g.:

> - **Per-purpose exclusivity (`one_of_domain`)** — an admin-curated, per-`purpose`
>   flag governing whether a subject may hold more than one tag of that purpose at a
>   time; enforced at the DB (trigger), API, and GUI layers

Match the existing bullets' terse, single-line style.

### 2. New `docs/decisions/tags-one-of-domain.md`

Mirror `docs/decisions/tags-limited-immutability.md`'s section structure exactly
(`## Purpose and scope`, `## Context`, `## Decision`, `## Consequences`), with
subsections as needed. Content to include (source from `plan/overview.md`'s "Design
decisions" section, which already contains the full rationale — synthesize into
decision-record prose, do not just copy bullet points verbatim):

- **Purpose and scope**: what `one_of_domain` is, that "domain" in the original
  request meant `purpose` (not a new grouping concept), and what this record covers
  (schema shape, default, enforcement mechanism, API/GUI surface).
- **Context**: today's unconditional `tags_owner_subject_purpose_idx` unique index and
  why it needed to become conditional.
- **Decision**, with subsections for:
  - The `tag_purpose_policies` table shape and why it's separate from `tag_templates`
    (Design decision 1's full rationale, including the naming-collision note vs. the
    roadmap's unrelated, deferred `tag_qualifier_policies`).
  - The `false`-when-absent default (Design decision 2).
  - The trigger-based enforcement mechanism, including the SQLSTATE-reuse trick and
    the `pg_advisory_xact_lock` race-safety mitigation (Design decision 3) — this is
    the most novel part of the design and deserves the fullest treatment, matching how
    `tags-limited-immutability.md` gives its enforcement-mechanism section full code
    listings.
  - The API surface decisions: embedding `oneOfDomain` on every tag response instead of
    a new lookup endpoint, and the no-public-write-endpoint choice for the policy flag
    itself (Design decision 4).
  - The GUI decisions: treating the existing purpose `<select>`/free-form input as "the
    picker" (no new combo-box widget), and fixing the pre-existing client-side
    duplicate-purpose validation gap as part of this change (Design decision 5).
- **Consequences**: a column-mutability-style summary table or short list analogous to
  `tags-limited-immutability.md`'s, covering: `tags_owner_subject_purpose_idx` is no
  longer a uniqueness guarantee by itself (the trigger is now load-bearing for that);
  `tag_purpose_policies` rows are admin-managed only, no public write path; the
  trigger's advisory-lock mitigation is a documented, accepted tradeoff of the
  trigger-based approach vs. a true unique index.

### 3. `next-steps.md` updates

**Add to "## Pending manual verification (needs live stack / DB)"** (matching that
section's existing `make dev.start` smoke-test bullet style):

> - **`one_of_domain` smoke.** Seed a `tag_purpose_policies` row with
>   `one_of_domain = true` for a test purpose (e.g. `priority`), then:
>   - `POST /v1/tags {subject, purpose: "priority", value: "low"}` → 201.
>   - `POST /v1/tags {subject, purpose: "priority", value: "urgent"}` (same
>     owner/subject) → 409 `conflict`.
>   - `POST /v1/tags {subject, purpose: "team", value: "platform"}` (a purpose with no
>     policy row, defaulting to `one_of_domain = false`) followed by a second `team`
>     tag with a different value on the same subject → both 201.

**Add to "## Known carry-forward items (non-blocking)"**:

> - **`tag_purpose_policies` has no public write endpoint.**
>   `TagPurposePolicyService.Upsert` (`api/service/tag_purpose_policy.go`) is
>   internal/administrative only, mirroring `tag_templates`' existing `Upsert`
>   convention — no HTTP route calls it. Rows must be seeded out-of-band (direct SQL, a
>   future admin tool, or a consuming app's own startup hook) until/unless a curated
>   admin surface is designed. See `docs/decisions/tags-one-of-domain.md`.
> - **No `scope` dimension on `tag_purpose_policies`.** Unlike `tag_templates`,
>   `one_of_domain` is global per `purpose`, with no per-app/per-scope variant in this
>   phase. A future scoped variant would be a separate, not-yet-designed extension —
>   see `docs/project-roadmap.md`.

Adjust exact wording as needed to match the file's existing tone; verify against the
actual landed behavior from Phases 1–3 rather than copying this verbatim if anything
diverged during implementation.

### 4. `docs/project-roadmap.md` update

Add a bullet under "## Possible future goals" (after the existing two), e.g.:

> - **Possible future extension: a `domain` grouping above `purpose`.** Not
>   implemented now. `one_of_domain` (see `docs/decisions/tags-one-of-domain.md`)
>   governs exclusivity per individual `purpose` value; the feature request that
>   produced it originally used the word "domain" to describe this concept before
>   being clarified to mean `purpose` directly. If a future need arises to group
>   multiple distinct `purpose` values under a shared exclusivity domain (e.g. treating
>   `priority` and `urgency` as mutually exclusive with *each other*, not just
>   internally), that would be a separate, broader grouping concept layered above
>   today's per-`purpose` policy — not yet designed or committed. Distinct from the
>   already-listed, deferred `tag_qualifier_policies` idea above, which is about
>   open/closed *value*-catalog enforcement, not exclusivity cardinality.

## Validation

- `docs/decisions/tags-one-of-domain.md` exists, follows
  `docs/decisions/tags-limited-immutability.md`'s section structure
  (`## Purpose and scope`, `## Context`, `## Decision`, `## Consequences`), and is
  linked from `README.md`'s "Additional documentation" list (add a bullet there too,
  matching the existing `tags-limited-immutability.md` entry's format).
- `grep -n "one_of_domain\|One-Of-Domain\|One Of Domain" README.md` shows the new Core
  features bullet.
- `grep -n "tag_purpose_policies\|tag_qualifier_policies" docs/project-roadmap.md`
  shows both terms present and the new bullet distinguishing them.
- `next-steps.md` contains both new bullets described above, in the correct existing
  sections.
- No changes to `model/README.md`, `api/README.md`, or `gui/README.md` from this task
  (`git diff --stat` should not list them).
- Markdown structure sanity check: every new/edited doc still opens with a `##
  Purpose and scope` section where applicable (decision records), per this project's
  general documentation conventions (see the existing decision record and README for
  precedent) — no frontmatter added anywhere.

## References

- `plan/overview.md` — full "Design decisions" section (source content for the new
  decision record).
- `docs/decisions/tags-limited-immutability.md` — structural/format precedent.
- `README.md`, `next-steps.md`, `docs/project-roadmap.md` — files edited by this task.
- `model/README.md` — already updated in Phase 1 task
  `001-add-tag-purpose-policies-table.md`; not touched again here.

## Status

**Outcome:** succeeded. Date: 2026-08-02.

- `README.md` — added the `one_of_domain` Core-features bullet and an "Additional
  documentation" entry linking to the new decision record.
- `docs/decisions/tags-one-of-domain.md` — new decision record, mirroring
  `docs/decisions/tags-limited-immutability.md`'s section structure
  (`## Purpose and scope`, `## Context`, `## Decision`, `## Consequences`), sourced
  from `plan/overview.md`'s Design decisions 1–5 and cross-checked against the
  actual landed code (`model/migrations/0205_tags_one_of_domain.sql`,
  `api/service/tag_purpose_policy.go`, `api/service/tag.go`,
  `api/httpapi/tags.go`, `gui/src/TagEditor.tsx`, `gui/src/lib/api.ts`) — no
  divergence found between the plan and the landed behavior/field names.
- `next-steps.md` — added the `one_of_domain` smoke-test bullet under "Pending
  manual verification" and the two carry-forward bullets (no public write
  endpoint; no `scope` dimension) under "Known carry-forward items".
- `docs/project-roadmap.md` — added the "Possible future extension: a `domain`
  grouping above `purpose`" bullet under "## Possible future goals", distinguishing
  `one_of_domain`/`tag_purpose_policies` from the pre-existing, unrelated
  `tag_qualifier_policies` deferred idea.
- Validation: all checks in `## Validation` passed (see structured report).
- No changes to `model/README.md`, `api/README.md`, or `gui/README.md`.
