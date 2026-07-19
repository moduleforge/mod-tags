# Project Roadmap

## Purpose and scope

Long-term planning for `mod-tags`: major new features, architectural shifts, and
systemic updates beyond the active implementation plan in `plan/`. This document
records the tag-templates catalog's deliberately **deferred** extensions —
decisions the product owner made explicitly not to implement as part of the
catalog shipped now — so a future engineer understands them as committed-or-open
future direction, not forgotten scope, and not part of the current change.

## Roadmap overview

The near-term theme is widening the tag-templates catalog's `scope` column so it
can generalize beyond app-level scoping, and resolving how access control should
work once it does. A second, independent theme — turning the catalog from a
suggestion-only surface into an enforced constraint on tag entry — is a
longer-horizon possibility gated on future product need, not yet committed to a
version. `## Version 1.0` below covers the first, committed item; `## Possible
future goals` covers the two items that remain open or undecided.

## Version 1.0

### Generalize `tag_templates.scope` from `apps` to `entities`

**Not implemented now.** Planned for 1.0.0; not part of the tag-templates catalog
shipped in the current change.

`tag_templates.scope` currently targets mod-core's `apps` table (`apps.id`).
Because `apps` is a pure FK-anchor table keyed on `entities.id` — `apps.id` is
itself an `entities.id`, with no surrogate key of its own — the FK target can be
widened from `apps` to `entities` later without a breaking rename or data
migration: every existing `apps.id` value already is a valid `entities.id`. The
intent is to generalize scoping (for example, to support per-user scoping in
addition to per-app scoping) by widening the FK target of the **same column**,
rather than adding a separate `owner` column alongside it.

## Possible future goals

- **Open: access-control model for scoped templates.** Not implemented now, and
  **unresolved** — no approach has been chosen. Candidate approaches:
  - Access rules derived from `scope` itself: a user may read templates scoped
    to their own user-entity or to the app they are using, but not another
    user's.
  - Leave access control to application-specific rules outside `mod-tags`.

  The product owner deliberately wants to avoid making `TagTemplate` an
  `Entity`, to avoid the entity-based access-control machinery the
  [`tags` table](./decisions/tags-limited-immutability.md) uses, but has
  acknowledged the team "may end up making them entities anyway" if no other
  access model resolves this question. This item stays open until a decision is
  made; it is not a commitment to either approach.

- **Deferred: open/closed qualifier-set policy layer.** Not implemented now. A
  future `tag_qualifier_policies` table — added purely additively, with a
  `mode: open | closed` column — plus service-layer (not database-trigger)
  enforcement at both tag-create time and value-update time. Both the table and
  the enforcement are deferred until the product actually needs to restrict tag
  entry, rather than merely suggest options via the catalog.
