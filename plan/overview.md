# One-Of-Domain Guard — mod-tags Slice

## Purpose and scope

Add a read-only `oneOfDomain` flag to mod-tags' existing catalog endpoint `GET /tag-templates`, sourced from the already-seeded `tag_purpose_policies` table, so a client can determine whether a `purpose` is one-of-domain **before any real tag of that purpose exists on the subject**.

This is mod-tags' slice of a two-repo bug-fix plan (`one-of-domain-guard`). The companion repo, `app-mftodo`, consumes the new field to fix a regression in its task-creation tag combobox and adds a client-side selection/typing guard mirroring mod-tags' own `gui/src/TagEditor.tsx`. mod-tags' phases land first because app-mftodo's fix depends on this field existing.

**In scope:** `model/queries/tag_templates.sql` (plus regenerated `model/db/`), `api/service/tag_template.go`, `api/httpapi/tag_templates.go`, `api/openapi.fragment.yaml`, tests at the query/service/API layers, and the endpoint/response documentation in `README.md`, `AGENTS.md`, and `docs/decisions/tags-one-of-domain.md`.

**Explicitly out of scope — hard constraints:**

- **No write path.** No HTTP route that writes or upserts `tag_purpose_policies` may be added. `TagPurposePolicyServicer.Upsert` stays internal/administrative with no route calling it, exactly as `docs/decisions/tags-one-of-domain.md` records.
- **No change to `TagPurposePolicyServicer`'s access-control posture.** It keeps making no `Authorizer` call. The chosen design (below) does not touch `api/service/tag_purpose_policy.go` at all.
- **No DB migration.** `tag_purpose_policies`, its trigger, and its `Get` accessor already exist (`model/migrations/0205_tags_one_of_domain.sql`).
- **No seeding in mod-tags.** Consuming apps seed their own rows out-of-band; app-mftodo already seeds a `priority` row with `one_of_domain=true` via its own appsetup code.
- **No GUI change.** `gui/src/` has no wiring to the `tag_templates` catalog and none is added here.

## Current status

Plan created; no tasks executed. Phase 1 (`tag-templates-one-of-domain`) begins first and has no pre-conditions beyond a working `sqlc` install and the sibling-symlink/compose preflight described under [Toolchain preconditions](#toolchain-preconditions). Phase 2 (`doc-updates`) runs after Phase 1's implementation tasks land.

## Overview

### Clarified request

**What must change.** `GET /tag-templates?purpose=<p>[&scope=<uuid>]` currently returns a bare JSON array of `{purpose, value, label, color, sortOrder, scope}` objects. Each object gains a sibling `oneOfDomain` boolean carrying the `tag_purpose_policies.one_of_domain` value for that row's `purpose`, defaulting to `false` when no policy row exists.

**What must not change.** The route's shape (bare array, not an envelope), its authentication posture, its existing fields, its error paths, and every other mod-tags endpoint. There are no new error paths — this is an additive field on an already-existing 200 response.

**Success criteria.**

1. A client issuing `GET /tag-templates?purpose=priority` against a database where `tag_purpose_policies` holds `('priority', true)` receives rows carrying `"oneOfDomain": true`, with no tag of purpose `priority` existing anywhere.
2. The same request for a purpose with **no** `tag_purpose_policies` row receives `"oneOfDomain": false` — matching the default the DB trigger itself applies on `NOT FOUND`.
3. Existing consumers that ignore the new key are unaffected: response status, ordering, and all pre-existing keys are byte-identical.
4. `cd api && go test ./...` and `make -C model verify` pass; `cd api && go vet ./... && gofmt -l .` are clean.

### Design decisions

The [tag-templates response-shape analysis](./notes/tag-templates-response-shape.md) records the alternatives considered and the evidence behind each choice. In summary:

**D1 — Per-row `oneOfDomain`, not a response envelope.** Each element of the existing array gains the flag, rather than wrapping the array in `{templates: [...], oneOfDomain: bool}` or adding a `purposePolicies` map. This mirrors the module's own established wire convention: `api/httpapi/tags.go`'s `tagResponse` already carries a per-row `OneOfDomain bool \`json:"oneOfDomain"\``, applied uniformly across every tag-returning endpoint precisely so the wire shape stays consistent module-wide. It is also purely additive — an envelope would be a breaking change to a shipped endpoint.

Because `purpose` is a **required** query parameter on this route, every row in one response shares a single purpose and therefore a single flag value; the per-row repetition is redundant but harmless, and it is what keeps the change additive.

**Accepted limitation.** A purpose with zero `tag_templates` rows returns an empty array and therefore carries no flag. This is inside the stated requirement ("for every purpose that has any tag-template entries") and inside app-mftodo's actual need (its combobox is populated from this catalog, so a purpose it offers always has entries). It is recorded here so it is a known, deliberate boundary rather than an oversight.

**D2 — Source the flag in SQL, not through `TagPurposePolicyServicer`.** `ListTagTemplates` gains `LEFT JOIN tag_purpose_policies tpp ON tpp.purpose = tt.purpose` and selects `COALESCE(tpp.one_of_domain, false) AS one_of_domain`. This is verbatim the pattern `model/queries/tags.sql` already uses on its own `:many` queries (`ListTagsBySubjectEntityID`, `SearchTags`) and the `:one` `GetTagByEntityUUID`. It keeps the read to a single query with no N+1, and — decisively, given the hard constraint above — it means `api/service/tag_purpose_policy.go` is not touched at all, so that service's access-control posture is unchanged by construction rather than by review.

**D3 — No migration, no new query file, no new route.** Only the existing `ListTagTemplates` query changes. `model/db/` is regenerated with `sqlc generate`.

### Phase 1 — Tag-Templates One-Of-Domain Flag

Delivers the flag end to end, bottom up. Tasks 001 → 002 are a strict chain; 003 and 004 both depend on 002 and are **parallel-eligible with each other**; 005 depends on 003.

| # | Task | Layer | Depends on |
|---|---|---|---|
| 001 | Add One Of Domain To List Query | `model/queries/`, regenerated `model/db/` | — |
| 002 | Thread One Of Domain Through Service | `api/service/` | 001 |
| 003 | Expose One Of Domain In Api Response | `api/httpapi/`, `api/openapi.fragment.yaml` | 002 |
| 004 | Add Catalog One Of Domain Integration Test | `api/service/` (build tag `integration`) | 002 |
| 005 | Update Module Endpoint Docs | `README.md`, `AGENTS.md` | 003 |

- **001** rewrites `ListTagTemplates` in `model/queries/tag_templates.sql` with the `LEFT JOIN` + `COALESCE` pattern and regenerates `model/db/tag_templates.sql.go` via `sqlc generate`, adding `OneOfDomain bool` to `ListTagTemplatesRow`.
- **002** adds `OneOfDomain bool` to `service.TagTemplate`, maps it in `hydrateTagTemplate`, extends the `mockTagQuerier` seeding helper in `api/service/mock_test.go`, and adds service-layer unit tests for the seeded-`true` and no-policy-row-`false` cases.
- **003** adds `OneOfDomain bool \`json:"oneOfDomain"\`` to `tagTemplateResponse`, maps it in `toTagTemplateResponse`, updates the `TagTemplate` schema in `api/openapi.fragment.yaml`, and adds handler tests asserting the JSON key.
- **004** adds a DB-backed integration test proving the real query returns `true` for a seeded purpose and `false` for an unseeded one, using the existing shared harness.
- **005** updates the endpoint/response documentation in the module `README.md` and `AGENTS.md`.

### Phase 2 — Documentation Updates

One task reconciling mod-tags' architecture-of-record — `docs/decisions/tags-one-of-domain.md` (whose "API surface" section currently states the flag reaches clients only via tag-returning responses) and `docs/project-roadmap.md` — with the API surface Phase 1 lands.

### Toolchain preconditions

Every implementing agent needs these; they are repeated in each task document that depends on them.

- **Sibling symlinks.** `api/go.mod` and `model/Makefile` use sibling-relative paths to `mod-core`. Inside a task worktree these do not resolve until `bash scripts/link-siblings.sh` (also run by `make preflight`) has planted the symlinks. Run it first from the worktree root.
- **Composed schema.** `model/schema/migrations/` is **gitignored** and rebuilt by `make -C model compose` from mod-core's migrations plus this module's own. `sqlc generate` and `sqlc compile` both read it, so a fresh worktree must compose before either will work, and a *stale* copy fails just as loudly (`relation "tag_purpose_policies" does not exist`, reported against every existing query, not just the new one). `make -C model build` runs `compose` then `sqlc generate`.
- **sqlc version.** `model/db/` was generated by **sqlc v1.31.1**. Confirm `sqlc version` matches before regenerating; a different version rewrites the `// versions:` banner in every generated file, producing a wide diff unrelated to this change.
- **Test commands.** Prefer `cd api && go test ./...` and `make -C model verify` over the root `make test`, which additionally requires Bun and a `gui/` typecheck that this change does not affect.
