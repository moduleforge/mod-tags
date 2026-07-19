# Update Architecture Docs

## Purpose and scope

Reconcile the module's architecture- and contributor-facing documentation with the
new `tag_templates` catalog table and the `GET /v1/tag-templates` endpoint
delivered in Phase 1. Runs after the Phase 1 implementation tasks land. Follow the
`update-architecture-docs` task procedure at
`plugins/flow/task-procedures/update-architecture-docs/SKILL.md`.

## Requirements

The following Phase 1 implementation task documents surfaced the architectural
implications this task reconciles (both completed by the time this phase runs):

- `plan/phase-01-tag-templates-catalog/001-model-schema-and-queries.md` — adds the
  new persisted `tag_templates` table (significant tracked state) and its
  cross-module FK to mod-core's `apps`.
- `plan/phase-01-tag-templates-catalog/002-api-list-endpoint.md` — adds the new
  public `GET /v1/tag-templates` endpoint (public API boundary change).

Review and update where needed (name each file explicitly; update only where the
change is actually reflected):

- `AGENTS.md` — the "Router mounting" route list (add `GET /tag-templates` and a
  one-line note that it is an open, catalog-only read with no per-row authz), the
  "Key files and directories" table if it enumerates schema concepts, and the
  migration-range note if it references the current highest migration.
- `README.md` — if it lists routes or module capabilities, add the tag-templates
  catalog.
- `model/README.md` — mentions the tables this module holds (currently `tags`,
  `entity_tags`); add `tag_templates` and note the sqlc-mirror `apps` dependency.
- `api/openapi.fragment.yaml` — verify Phase 1 added the `/tag-templates` path and
  `TagTemplate` schema; reconcile the fragment `description`/`info` if it
  enumerates owned routes.
- The cross-cutting architecture docs under `docs/mf-standards/architecture/`
  (e.g. `db-considerations.md`, `entity-typing.md`) **only if** they enumerate
  module tables or would be contradicted by a non-entity catalog table — note,
  do not force edits into the shared `mf-standards` submodule unless clearly
  warranted; prefer module-local docs. Flag rather than edit if a shared-doc
  change is ambiguous.
- Confirm `docs/project-roadmap.md` (created in Phase 1 task 003) is linked from a
  discoverable location (e.g. `AGENTS.md` or `README.md`) per the documentation
  standards' link-chain rule.

`role_doc: plugins/flow/references/roles/architect-data.md` — the primary
architectural implication is a new persisted table and a cross-module schema FK;
the accompanying endpoint is a thin read over it.

## Validation

- `AGENTS.md`, `README.md`, and `model/README.md` were reviewed; the route list
  and table inventory reflect `tag_templates` / `GET /v1/tag-templates` where those
  docs enumerate them.
- `docs/project-roadmap.md` is reachable via at least one link from a top-level
  doc.
- No doc claims `tag_templates` is an entity or that templates are enforced against
  tag creation.
- Any shared-`mf-standards` change that was deemed out of scope is explicitly
  flagged in the task report rather than silently skipped.

## References

- `plugins/flow/task-procedures/update-architecture-docs/SKILL.md` — the procedure
  to follow.
- [Schema grounding notes](../notes/schema-grounding.md) — the settled design the
  docs must reflect.
- Phase 1 task docs (listed above) — the source of the architectural changes.
</content>
