# Tag Templates Catalog

## Purpose and scope

Add a **tag-templates catalog** to mod-tags: a new `tag_templates` table plus a
single open read endpoint that lets consuming applications suggest
purpose/value/label/color options for a combo-box, optionally scoped to an app.
This is a **schema + API + docs change only**. It is grounded in an already-settled
opus-high design consultation and product-owner Q&A; those decisions are not
reopened here (see [schema grounding notes](./notes/schema-grounding.md) for how
each was confirmed against the actual schema).

Hard constraints (carried verbatim into every task):

- **No changes to the existing `tags` table** or its create/update/delete code
  paths. This is a read-side/catalog-only addition.
- **No seeding** — ship `tag_templates` with zero rows. Seeding is a
  consuming-application concern.
- **No enforcement** of templates against tag creation/mutation, no `'*'`
  wildcard sentinel, no strict/loose runtime flag.
- **`TagTemplate` must not become an `Entity`** — it is a plain table with its own
  `BIGSERIAL` PK, deliberately not wired into `entities` or the type/entity
  resolvers, to keep it out of the entity-based access-control machinery.

## Current status

Fresh plan. Starts at Phase 1 (Tag Templates Catalog). No pre-conditions beyond
the existing mod-tags/mod-core codebase. Phase 2 (Documentation Updates) is added
by the planner's architectural-implications check because the change adds a new
public endpoint and a new persisted table.

Key pre-resolved design decisions (full detail in
[schema grounding](./notes/schema-grounding.md)):

- `scope BIGINT REFERENCES apps(id) ON DELETE CASCADE`, nullable
  (`NULL` = global). `apps` is mod-core's FK-anchor table keyed on `entities.id`;
  the `?scope=<app id>` param is the app's **public UUID**, resolved to the
  internal id in the handler.
- Uniqueness via **two partial unique indexes** —
  `(scope, purpose, value) WHERE scope IS NOT NULL` and
  `(purpose, value) WHERE scope IS NULL` — because a plain unique index will not
  dedupe global (`NULL`-scope) rows since `NULL != NULL`.
- **Build-time gotcha:** the sqlc schema mirror (`model/schema/migrations/`) does
  **not** currently contain mod-core's `apps` table, so the `apps` definition must
  be mirrored in before `sqlc generate` will resolve the new FK. The runtime goose
  migrations do not get an `apps` copy (mod-core supplies it at runtime).
- Endpoint: `GET /v1/tag-templates?purpose=<p>[&scope=<app-uuid>]`. No `scope` →
  globals only; `scope` set → globals + that app's rows. Open read (no `Authorizer`
  / access-function row filtering); registered alongside existing tag routes under
  the manifest's `authenticated` scope. Bare-array response of
  `{purpose, value, label, color, sortOrder, scope}`; errors via
  `apiresp.WriteError`.

## Overview

The change is one coherent feature slice with no research gaps, so it is a single
implementation phase (plus the standard documentation-updates phase). Tasks are
ordered by dependency; the roadmap doc is independent and parallel-eligible.

### Phase 1 — Tag Templates Catalog

- **001 — Model: `tag_templates` schema + sqlc queries** *(sonnet-high)*. Add
  goose migration `0204_tag_templates.sql` (table, two partial unique indexes, a
  supporting `(purpose, scope)` index, `set_updated_at` trigger); mirror it into
  `model/schema/migrations/` **and add mod-core's `apps` table to that sqlc
  mirror** so the FK resolves; add `model/queries/tag_templates.sql` (a `narg`
  scope list query with a `LEFT JOIN entities` for the scope UUID); run
  `sqlc generate`. Validate with `make verify` and the shadow-DB lint.
  *Blocks 002.*
- **002 — API: `GET /v1/tag-templates` list endpoint** *(sonnet-high)*. Add a
  read-only service method (resolve `scope` app-UUID → internal id, call the
  generated query, map rows to a UUID-only response), a thin handler, register the
  route in `router.go` (`NewRouter` + `RegisterRoutes`), and add the path +
  schema to `api/openapi.fragment.yaml`. Open read: no `Authorizer` call, no
  access-function filtering. Errors via `apiresp.WriteError`. *Depends on 001.*
- **003 — Docs: `docs/project-roadmap.md`** *(sonnet-med)*. Create the new
  roadmap doc recording the explicitly NOT-now forward-looking notes (1.0.0 scope
  FK migration from `apps` → `entities`; the open scoped-template access-control
  question; the deferred open/closed qualifier-policy layer and service-layer
  enforcement). Independent of 001/002 — **parallel-eligible**.

### Phase 2 — Documentation Updates

- **001 — Update architecture docs** *(sonnet-high)*. Reconcile the module-facing
  docs (AGENTS.md route list, README, OpenAPI-fragment cross-refs, and the
  cross-cutting architecture docs where the new table/endpoint are relevant) with
  the new catalog table and endpoint delivered in Phase 1. Runs after Phase 1
  lands.

### Dependencies and parallelism

- 001 → 002 (API consumes the generated queries and the resolved schema).
- 003 is independent of 001/002 and may run concurrently with them.
- Phase 2/001 runs after Phase 1 completes.
</content>
