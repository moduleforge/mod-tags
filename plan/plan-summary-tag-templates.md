# Plan Summary: Tag Templates Catalog

## What was planned and why

Add a **tag-templates catalog** to mod-tags: a new `tag_templates` table plus a
single open read endpoint (`GET /v1/tag-templates`) that lets consuming
applications suggest purpose/value/label/color options for a combo-box,
optionally scoped to an app. This was a schema + API + docs change only,
grounded in an already-settled opus-high design consultation and
product-owner Q&A (decisions not reopened in this plan; see
`plan/notes/schema-grounding.md`).

Hard constraints carried into every task:

- No changes to the existing `tags` table or its create/update/delete code
  paths — read-side/catalog-only addition.
- No seeding — `tag_templates` ships with zero rows; seeding is a
  consuming-application concern.
- No enforcement of templates against tag creation/mutation, no `'*'`
  wildcard sentinel, no strict/loose runtime flag.
- `TagTemplate` must not become an `Entity` — a plain table with its own
  `BIGSERIAL` PK, deliberately kept out of the entity-based access-control
  machinery.

The plan was a single coherent feature slice with no research gaps: one
implementation phase (Phase 1) plus the standard documentation-updates phase
(Phase 2, triggered by the planner's architectural-implications check because
the change adds a new public endpoint and a new persisted table).

## What shipped

### Phase 1 — Tag Templates Catalog

- **001 — Model: `tag_templates` schema + sqlc queries** (sonnet-high).
  Implemented the catalog table exactly per the task doc's SQL: goose
  migration `0204_tag_templates.sql` with the two partial unique indexes
  (`(scope, purpose, value) WHERE scope IS NOT NULL` and
  `(purpose, value) WHERE scope IS NULL`), a supporting `(purpose, scope)`
  index, and a `set_updated_at` trigger; the `ListTagTemplates` sqlc query;
  and the sqlc-regenerated `tag_templates.sql.go` plus additive
  `models.go`/`querier.go`. One substantive deviation: the literal
  hand-add-`apps`-to-committed-mirror instruction was not followed because
  `model/schema/migrations/` is gitignored, build-time-composed output that
  already picks up mod-core's real `apps` table automatically via
  `make compose` — confirmed empirically. `make verify` and `make lint`
  fail for reasons demonstrably pre-existing and unrelated (reproduced
  against the unmodified baseline); equivalent manual validation was
  substituted and passed. `tags`-related files untouched throughout.
  Commit `5ab5fc2`, merged via `46deada`.
- **002 — API: `GET /v1/tag-templates` list endpoint** (sonnet-high).
  Implemented as a thin open-read endpoint per the task doc: a read-only
  service method (resolve `scope` app-UUID → internal id, call the
  generated query, map rows to a UUID-only response), a handler, route
  registration in `router.go` (exactly two lines added), and the path +
  schema added to `api/openapi.fragment.yaml` (validated clean).
  `tags.go`/`tag.go`/`errors.go`/`subject_tags.go` untouched. All
  validation checks pass. Commit `d227ced`, merged via `b8983ed`.
- **003 — Docs: `docs/project-roadmap.md`** (sonnet-med, parallel-eligible).
  Authored `docs/project-roadmap.md` per the Project Roadmap Standards,
  recording the three deferred/open items (1.0.0 scope FK migration from
  `apps` → `entities`; the open scoped-template access-control question;
  the deferred open/closed qualifier-policy layer and service-layer
  enforcement). All four validation checks pass. Commit `ecabeac`, merged
  via `f5152c2`.
- **Immediate fix (manager-dispatched, no task doc)** — the Phase 1
  phase-review security lens found the new `GET /v1/tag-templates` handler
  lacked an actor-presence check; fixed directly by the manager
  (commit `ed08336`, merged via `5c6480d`).

### Phase 2 — Documentation Updates

- **001 — Update architecture docs** (sonnet-high). Reconciled AGENTS.md,
  README.md, `model/README.md`, and `api/openapi.fragment.yaml` with the
  Phase 1 `tag_templates` table and `GET /v1/tag-templates` endpoint,
  including the post-002 actor-presence fix. Renamed README's "Module
  documentation" section to "Additional documentation" and linked
  `docs/project-roadmap.md`, resolving the Phase 1 followup (`sIKF`) about
  the missing link and non-standard heading name. Reviewed the shared
  mf-standards submodule per the task doc's instruction and made no edits
  there. All validation checks pass. Commit `dd0be34`, merged via
  `661fd77`.
- **Immediate fixes (manager-dispatched, no task doc)** — the Phase 2
  phase-review found a stale route comment in `moduleforge.module.yaml` and
  a missing 401 response in `api/openapi.fragment.yaml`; fixed directly by
  the manager (commit `e90007b`, merged via `612b1f2`), followed by a
  trivial README wording fix (commit `2970c43`).

## Key decisions

- **`scope` is a nullable FK to `apps(id)` with `ON DELETE CASCADE`**;
  `NULL` means global. `apps` is mod-core's FK-anchor table keyed on
  `entities.id`; the `?scope=<app id>` query param is the app's public
  UUID, resolved to the internal id inside the handler/service layer, not
  passed through raw.
- **Two partial unique indexes instead of one plain unique index**, because
  a plain unique index would not dedupe global (`NULL`-scope) rows, since
  SQL `NULL != NULL`.
- **`model/schema/migrations/` mirror is gitignored/build-composed**, not a
  file to hand-edit — task 001 discovered `make compose` already pulls in
  mod-core's real `apps` table, so no manual mirror edit was needed despite
  the task doc's literal instruction to add one. Runtime goose migrations
  do not get an `apps` copy; mod-core supplies it at runtime.
- **Open read, no `Authorizer` / access-function row filtering** — the
  endpoint is registered alongside existing tag routes under the
  manifest's `authenticated` scope, but still requires a present actor
  (the actor-presence gap was caught and fixed post-Phase-1 by
  phase-review, commit `ed08336`).
- **`TagTemplate` is deliberately not an `Entity`** — a plain table with
  its own `BIGSERIAL` PK, kept out of the entity-based access-control
  machinery, per the plan's hard constraints.
- **No seeding, no tag-creation enforcement, no `'*'` wildcard, no
  strict/loose flag** — all deferred to consuming applications /
  future work, per the plan's hard constraints; the deferred items are
  now formally tracked in `docs/project-roadmap.md` (task 003).
- **Shared mf-standards submodule not edited** — Phase 2 reviewed
  `db-considerations.md`, `entity-typing.md`, and `authorization-design.md`
  and found nothing warranting a change; notably, `authorization-design.md`
  states a blanket all-reads-must-be-authorized rule that this module's
  open-read design deviates from, but the task doc directed against
  forcing edits into the shared, separately-repo'd submodule on ambiguous
  cases.

## Follow-up items

Carried forward from `plan/followups.yaml` (items tagged
`tag-templates-catalog` or `doc-updates`; the `go-apiresp-migration` and
`gui-error-handling`-tagged items in the same file belong to a different,
unrelated plan and are not reproduced here):

- **Open design questions left as-is per task instruction** (id `6By8`):
  (1) whether `/tag-templates` should be truly public/unauthenticated
  rather than sitting under the shared `authenticated` manifest group; (2)
  whether `?scope=<uuid>` should return scoped-only rows instead of
  globals+scoped. Both are already recorded as open questions in
  `docs/project-roadmap.md`.
- **Consider pagination on `ListTagTemplates`** (id `4VMG`, phase-review
  efficiency lens, medium confidence): no `LIMIT`/pagination today; low
  risk while `tag_templates` has no write path and rows are
  catalog/admin-curated, but add `LIMIT`/`OFFSET` or cursor pagination
  before a real payload-size incident if a future write path lets a
  single `purpose` grow unbounded.
- **Confirm `/tag-templates` call frequency** (id `B1Ol`, phase-review
  efficiency lens, low confidence): the endpoint likely backs UI
  combo-box population (a common every-render pattern) but does two DB
  round trips per call with no `Cache-Control`/`ETag` support and no
  server-side memoization. Confirm expected call volume with the frontend
  team; add caching if it turns out to be a hot path.
- **`cd model && make verify` fails on a pre-existing defect** (id `uq0y`):
  `goose -dir migrations validate` chokes on the non-SQL
  `migrations/migrate.go` helper file; reproduced against the unmodified
  checkout, out of this plan's scope, but worth its own follow-up if
  `make verify` is relied on in CI.
- **`cd model && make lint` fails in sandboxed environments** (id `dNvT`):
  `scripts/shadow-db-lint.sh` fails on Docker container-IP connectivity in
  this sandbox (works with host port-forwarding); reproduced against the
  unmodified checkout — an environment limitation, not a migration defect.
- **Worktree Go build/test path breakage, workaround pattern** (id
  `WtUT`): any task needing `cd api && go build|test|vet ./...` inside a
  mod-tags worktree under `worktrees/<task>/` hits the
  `../../mod-core/{api,model}` replace-path breakage first documented in a
  prior (unrelated) plan's followups; the `GOWORK`-scratch-file workaround
  used here is a clean pattern worth standardizing on for subsequent
  `api/`-touching tasks.
- **`docs/project-roadmap.md` README link — already resolved** (id
  `sIKF`): flagged after Phase 1 task 003 as a missing link from README's
  "Additional documentation" section; Phase 2 task 001 resolved this
  (renamed the section and added the link), so no further action is
  needed, though the item remains listed in `followups.yaml` verbatim.
- **`authorization-design.md` deviation noted, not resolved** (id `rvMe`):
  the shared mf-standards submodule states a blanket all-reads-must-be-
  authorized rule that this module's open-read `/tag-templates` design
  deviates from at the module level; left unedited since the submodule
  lives in a separate repo and the task doc directed against forcing
  edits on ambiguous shared-doc cases. Worth a follow-up decision at the
  org/standards level.
- **Stale/unrelated doc issues noticed but out of scope** (ids `yzkH`,
  `QRHG`): `model/README.md`'s `entity_tags` table reference appears
  stale/pre-existing and unrelated to this change; `api/README.md` and
  `gui/README.md` are stale placeholders predating and unrelated to the
  tag-templates change. Both noticed during Phase 2 doc review and left
  untouched.

## Final Task State

# TODO

## Purpose and scope

Tracking document for the active plan.

## Tasks

### Phase 01 — Tag Templates Catalog

- [x] [001-model-schema-and-queries.md](./phase-01-tag-templates-catalog/001-model-schema-and-queries.md) — tier `sonnet-high` · branch `phase-01-task-01-model-tag-templates-schema-sql` · commit `5ab5fc2` · merge `46deada`
- [x] [002-api-list-endpoint.md](./phase-01-tag-templates-catalog/002-api-list-endpoint.md) — tier `sonnet-high` · branch `phase-01-task-02-api-tag-templates-list-endpoin` · commit `d227ced` · merge `b8983ed`
- [x] [003-project-roadmap-doc.md](./phase-01-tag-templates-catalog/003-project-roadmap-doc.md) — tier `sonnet-med` · branch `phase-01-task-03-docs-create-project-roadmap` · commit `ecabeac` · merge `f5152c2`

### Phase 02 — Documentation Updates

- [x] [001-update-architecture-docs.md](./phase-02-doc-updates/001-update-architecture-docs.md) — tier `sonnet-high` · branch `phase-02-task-01-update-architecture-docs` · commit `dd0be34` · merge `661fd77`
