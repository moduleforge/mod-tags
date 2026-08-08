# Plan Summary: one-of-domain-guard

## What was planned and why

This is mod-tags' slice of a two-repo bug-fix plan. A user reported a regression: creating a task in
app-mftodo, the tag combobox didn't exclude other `priority:*` tags after picking `priority:low` — the
first-ever tag of that purpose, template-sourced, no real tag existed yet — violating mod-tags' own
`one_of_domain` exclusivity rule. Investigation traced the root cause to mod-tags: `tag_purpose_policies`
(the real one-of-domain source of truth) had no HTTP route at all, so a consuming app's GUI could only
infer one-of-domain-ness from existing real `Tag` rows — invisible for a purpose's first-ever tag.

The plan added a read-only `oneOfDomain` boolean to mod-tags' existing catalog endpoint
`GET /tag-templates`, sourced from the already-seeded `tag_purpose_policies` table, so a client can
determine whether a `purpose` is one-of-domain **before any real tag of that purpose exists on the
subject**.

**In scope:** `model/queries/tag_templates.sql` (plus regenerated `model/db/`), `api/service/tag_template.go`,
`api/httpapi/tag_templates.go`, `api/openapi.fragment.yaml`, tests at the query/service/API layers, and
the endpoint/response documentation in `README.md`, `AGENTS.md`, and `docs/decisions/tags-one-of-domain.md`.

**Explicitly out of scope — hard constraints, all held:**

- No write path for `tag_purpose_policies` — `TagPurposePolicyServicer.Upsert` stays internal/administrative
  with no route calling it.
- No change to `TagPurposePolicyServicer`'s access-control posture — it keeps making no `Authorizer` call.
- No DB migration — `tag_purpose_policies`, its trigger, and its `Get` accessor already existed.
- No seeding in mod-tags — consuming apps seed their own rows out-of-band.
- No GUI change in mod-tags.

The companion repo, app-mftodo, consumes the new field to fix the regression in its own task-creation
tag combobox and adds a client-side selection/typing guard mirroring mod-tags' own `gui/src/TagEditor.tsx`.
mod-tags' phases landed first since app-mftodo's fix depends on this field existing (though app-mftodo's
own design made the field optional client-side, so implementation there did not strictly block on this
merge — only end-to-end verification did).

## What shipped

### Phase 1 — Tag-Templates One-Of-Domain Flag

**001 — Add One Of Domain To List Query** (`model/queries/tag_templates.sql`, regenerated `model/db/`)
Rewrote `ListTagTemplates` with `LEFT JOIN tag_purpose_policies tpp ON tpp.purpose = tt.purpose` and
`COALESCE(tpp.one_of_domain, false) AS one_of_domain` — the exact pattern `model/queries/tags.sql` already
used on its own `:many` queries. `UpsertTagTemplate` and its load-bearing leading comment (which sqlc
propagates into `querier.go`) were left untouched. Regenerated via `sqlc generate` (v1.31.1, matching the
pinned version); `ListTagTemplatesRow` gained a trailing `OneOfDomain bool` field. Branch
`plan/one-of-domain-guard-01-001`, commit `76566cb`, merged as `fd845e93`.

**002 — Thread One Of Domain Through Service** (`api/service/tag_template.go`, `mock_test.go`, `tag_template_test.go`)
Added `OneOfDomain bool` to `service.TagTemplate`, wired through `hydrateTagTemplate`; `hydrateUpsertedTagTemplate`
deliberately left at its zero value since `UpsertTagTemplateRow`'s `RETURNING` clause doesn't select the
column. The mock querier's `ListTagTemplates` was taught to derive the flag from its `policies` map,
simulating the join without mutating `seedTemplate`'s stored rows. `tag_purpose_policy.go` was read but
not modified, and the catalog read remains `Authorizer`-free. Three new service-layer tests added. Branch
`plan/one-of-domain-guard-01-002`, commit `263890b`, merged as `3d790e85`.

**003 — Expose One Of Domain In Api Response** (`api/httpapi/tag_templates.go`, `api/openapi.fragment.yaml`, tests)
Added `OneOfDomain bool \`json:"oneOfDomain"\`` to `tagTemplateResponse`, matching `tags.go`'s existing
per-tag `OneOfDomain` key spelling exactly, no `omitempty`. Updated the OpenAPI fragment's `TagTemplate`
schema (marked `oneOfDomain` required). Two new handler tests cover true/false cases plus a bare-array-shape
assertion. Branch `plan/one-of-domain-guard-01-003`, commit `12bad1e`, merged as `f3f43856`.

**004 — Add Catalog One Of Domain Integration Test** (new `api/service/tag_template_one_of_domain_integration_test.go`)
A live-DB test covering seeded-true, explicit-false, and no-policy-row cases in one session, seeding
catalog rows without ever creating a real `tags` row — the direct proof that the flag is readable before
any tag of the purpose exists. The live-DB run itself could not execute in the implementation sandbox due
to a local Postgres port-5432 conflict (native Homebrew Postgres shadowing the intended Docker container);
the test's compile-cleanliness (`go vet -tags=integration`) was confirmed instead, an outcome the task's
own Assumptions anticipated as acceptable. Branch `plan/one-of-domain-guard-01-004`, commit `ee59470`,
merged as `6431f2f6`.

**005 — Update Module Endpoint Docs** (`README.md`, `AGENTS.md`, `next-steps.md`)
Documented the new `oneOfDomain` field on `GET /tag-templates`, preserving the "open, catalog-only,
no-per-row-authorization" characterization and tightening the `next-steps.md` write-endpoint carry-forward
item to distinguish "readable" from "writable." Branch `plan/one-of-domain-guard-01-005`, commit `644a1f0`,
merged as `c8cbc0d5`.

**Phase 1 gate:** correctness, efficiency, and security lenses (decomposed dispatch, triggered by
`architectural_impact: true` on task 003) all returned no findings. Architecture check: not-applicable
(no project-wide architecture doc). Link chain: intact.

### Phase 2 — Documentation Updates

**001 — Update Architecture Docs** (`docs/decisions/tags-one-of-domain.md`)
Reconciled the decision record with Phase 1's landed API surface. Added a dated addendum under "API
surface" explaining catalog-time exposure as answering a different question than the existing per-tag
"occupied" field ("is this purpose exclusive at all" vs. "is it occupied on this subject"), explicitly
noting no new endpoint was added and that the original "occupied" reasoning stands unrevised. Tightened
the "no public write endpoint" bullet to scope its "no HTTP route" claims to the write path only, and
clarified the read path reaches clients via the SQL join, not via any route calling
`TagPurposePolicyServicer.Get`. Added a `Consequences` table row for the catalog read's new visibility
of the flag. `docs/project-roadmap.md` was reviewed and confirmed to need no change. Branch
`plan/one-of-domain-guard-02-001`, commit `52e15f6`, merged as `8aa9a510`.

**Phase 2 gate:** combined review (correctness + baseline security, doc-only diff) returned no findings.

## Key decisions

- **Per-row `oneOfDomain`, not a response envelope.** Mirrors the module's established wire convention
  (`tags.go`'s `tagResponse.OneOfDomain`) and stays additive — an envelope would have been a breaking
  change to a shipped endpoint.
- **Source the flag in SQL, not through `TagPurposePolicyServicer`.** Keeps the read to a single query
  with no N+1, and — decisively, given the hard constraints — means `api/service/tag_purpose_policy.go`
  is untouched by construction, so that servicer's access-control posture is unchanged by construction
  rather than by review.
- **No migration, no new query file, no new route.** Only the existing `ListTagTemplates` query changed.
- **Accepted limitation carried forward.** A purpose with zero `tag_templates` rows returns an empty array
  and therefore carries no flag — inside the stated requirement and inside app-mftodo's actual need.

## Follow-up items

1. **Live-DB integration test could not execute in this sandbox.** `api/service/tag_template_one_of_domain_integration_test.go`
   (and, by the same constraint, any sibling integration test in the package) could not run against a
   real Postgres: a native Homebrew Postgres on `127.0.0.1`/`[::1]:5432` shadows the
   `users-module-postgres` Docker container's own port-5432 forward, and the container's direct
   Docker-network IP times out from the macOS host. If a genuinely live-DB-verified run is required, the
   environment needs port 5432 not double-bound (stop the local Homebrew Postgres, or run the suite
   inside a container on the same Docker network). Outside this plan's scope to fix.

## Final Task State

# TODO

## Purpose and scope

Tracking document for the active plan.

## Tasks

### Phase 01 — Tag-Templates One-Of-Domain Flag

- [x] [001-add-one-of-domain-to-list-query.md](./phase-01-tag-templates-one-of-domain/001-add-one-of-domain-to-list-query.md) — tier `sonnet-med` · branch `plan/one-of-domain-guard-01-001` · commit `76566cb` · merge `fd845e934ec1308318e8b76f5c9d517d81338778`
- [x] [002-thread-one-of-domain-through-service.md](./phase-01-tag-templates-one-of-domain/002-thread-one-of-domain-through-service.md) — tier `sonnet-med` · branch `plan/one-of-domain-guard-01-002` · commit `263890b` · merge `3d790e85213dd21633c38e192eab035f5d539f66`
- [x] [003-expose-one-of-domain-in-api-response.md](./phase-01-tag-templates-one-of-domain/003-expose-one-of-domain-in-api-response.md) — tier `sonnet-med` · branch `plan/one-of-domain-guard-01-003` · commit `12bad1e` · merge `f3f43856ac85106773d7455e2113ba82ad86c8a2`
- [x] [004-add-catalog-one-of-domain-integration-test.md](./phase-01-tag-templates-one-of-domain/004-add-catalog-one-of-domain-integration-test.md) — tier `sonnet-med` · branch `plan/one-of-domain-guard-01-004` · commit `ee59470` · merge `6431f2f691dfbba3ea6937e3362fbd713c0792e3`
- [x] [005-update-module-endpoint-docs.md](./phase-01-tag-templates-one-of-domain/005-update-module-endpoint-docs.md) — tier `sonnet-low` · branch `plan/one-of-domain-guard-01-005` · commit `644a1f0` · merge `c8cbc0d58100144dbaebb8503b7bda9516c8d2dd`

### Phase 02 — Documentation Updates

- [x] [001-update-architecture-docs.md](./phase-02-doc-updates/001-update-architecture-docs.md) — tier `sonnet-high` · branch `plan/one-of-domain-guard-02-001` · commit `52e15f6` · merge `8aa9a5103cb131d0350d1768c088abec813c30d8`
