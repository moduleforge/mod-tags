# Docs: Create Project Roadmap

## Purpose and scope

Create `docs/project-roadmap.md` recording the forward-looking, explicitly
**NOT-implemented-now** decisions and open questions surrounding the tag-templates
catalog. This is a documentation-only task, independent of tasks 001/002 and
**parallel-eligible** with them. Invoke the standard `implement-task` procedure
(technical-writer / documentation lens).

Before writing, confirm no differently-named roadmap doc already exists under
`docs/` (verified during planning: `docs/` holds only `CLAUDE.md`, `decisions/`,
and the `mf-standards/` submodule — no roadmap doc; `next-steps.md` at the repo
root is implementation residue, not a forward roadmap). If a roadmap doc has since
appeared, extend it rather than duplicating.

## Requirements

Author `docs/project-roadmap.md` with a standard `## Purpose and scope` opening
section, then record the following as clearly-labeled **future / not-now** items.
Every item must be unambiguously marked as not implemented in the current change.

1. **Planned 1.0.0 scope-FK generalization.** Record the intent to migrate
   `tag_templates.scope`'s FK target from `apps` (mod-core) to `entities`
   (mod-core) — the **same column**, generalized later (e.g. to support per-user
   scoping) rather than adding a separate `owner` column, so no breaking
   rename/migration is required. Note that `apps.id` is itself an `entities.id`
   today, which is what makes this a widening of the FK target rather than a data
   migration.

2. **Open: access-control model for scoped templates.** Record as **open and
   unresolved** (do not resolve it). Candidate approaches:
   - (a) access rules derived from `scope` itself — a user may read templates
     scoped to their own user-entity or to the app they are using, but not another
     user's;
   - (b) left to application-specific rules outside mod-tags.
   Note the product owner deliberately wants to avoid making `TagTemplate` an
   `Entity` (to avoid the entity-based access-control machinery the `tags` table
   uses), but acknowledged the team "may end up making them entities anyway" if no
   other access model resolves this.

3. **Deferred: open/closed qualifier-set policy layer.** Carry forward, also
   **not implemented now**: a future `tag_qualifier_policies` table with a `mode:
   open | closed`, added purely additively; and service-layer (not DB-trigger)
   enforcement at both tag-create and value-update time. Both are deferred until
   the product needs to actually restrict tag entry rather than merely suggest
   options via the catalog.

Frame the doc so a future engineer understands these are the *deliberately
deferred* extensions of the catalog shipped now — not commitments and not part of
this change. Keep it consistent with the repo's documentation style and link to
`docs/decisions/tags-limited-immutability.md` where the "not an entity" / access
posture is relevant context.

## Validation

- `docs/project-roadmap.md` exists, opens with `## Purpose and scope`, and covers
  all three items above, each explicitly marked as future / not-implemented-now.
- The scoped-template access-control item is recorded as **open** (no resolution
  asserted).
- Markdown lints/renders cleanly (no broken relative links; the
  `tags-limited-immutability.md` link resolves).
- No pre-existing roadmap doc was overwritten or duplicated.

## References

- The change request's "New document required" section (verbatim source of the
  three items).
- `docs/decisions/tags-limited-immutability.md` — related decision-record style
  and the "not an entity" context.
- [Schema grounding notes](../notes/schema-grounding.md) — confirms no existing
  roadmap doc and the shipped-now scope FK design these notes evolve.

## Status

**Outcome:** succeeded. **Date:** 2026-07-18.

Created `docs/project-roadmap.md` per the Flow Project Roadmap Standards layout
(`# Project Roadmap` / `## Purpose and scope` / `## Roadmap overview` /
`## Version 1.0` / `## Possible future goals`). Confirmed via `find docs
-iname "*roadmap*"` that no roadmap doc existed before this task and none was
overwritten.

- Item 1 (scope-FK generalization `apps` → `entities`) placed under
  `## Version 1.0` as a `###` subsection, explicitly marked "Not implemented
  now. Planned for 1.0.0."
- Item 2 (open access-control model) and item 3 (deferred qualifier-set policy
  layer) placed under `## Possible future goals`, each explicitly marked "Not
  implemented now"; item 2 is worded as unresolved with no approach chosen.
- Linked `docs/decisions/tags-limited-immutability.md` inline on the `tags`
  table mention in the access-control item, per the requirement; link target
  confirmed to exist and resolve (relative path `./decisions/tags-limited-immutability.md`
  from `docs/project-roadmap.md`).

Validation: all four `## Validation` checks passed (see the structured report).

**Flagged for manager:** `docs/project-roadmap.md` is not yet linked from
`README.md`'s "Additional documentation" section, which the [README Document
Standards](flow-mcp:d) and [Project Roadmap Standards — Conventions](flow-mcp:s)
require for the link-chain rule to hold at publication time. This task's
`## Requirements` did not name a README edit, and README.md is not otherwise
part of this task's diff, so per the "smallest correct change" principle no
README edit was made here. Phase 2 task 001 ("Update architecture docs",
which already reconciles README with the new table/endpoint) is the natural
place to add this link before publication.
</content>
