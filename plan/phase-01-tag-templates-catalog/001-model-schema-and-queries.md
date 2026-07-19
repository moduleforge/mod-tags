# Model: Tag Templates Schema And Sqlc Queries

## Purpose and scope

Add the `tag_templates` catalog table to the mod-tags data model: a goose
migration, its sqlc schema-mirror copy (plus the `apps` FK-target table the mirror
is currently missing), an sqlc query file, and the regenerated Go query code.
This is a pure additive schema change. **The existing `tags` table and its
migrations/queries are not touched.** Invoke the standard `implement-task`
procedure with the SQL Developer / Go Developer standards.

`tag_templates` is deliberately **not** an entity: it has its own `BIGSERIAL`
primary key, is not inserted into `entities`, has no `types` seed, and is not
wired to the type/entity resolvers.

## Requirements

### 1. Runtime migration — `model/migrations/0204_tag_templates.sql`

`0204` is the next free number in this module's 0200–0299 range (existing:
0200–0203, 0299). Author a goose `+goose Up` / `+goose Down` migration creating:

```sql
-- +goose Up
CREATE TABLE tag_templates (
  id          BIGSERIAL PRIMARY KEY,
  scope       BIGINT REFERENCES apps(id) ON DELETE CASCADE,       -- NULL = global template
  purpose     TEXT NOT NULL CHECK (char_length(purpose) <= 512),
  value       TEXT NOT NULL CHECK (char_length(value) <= 512),
  label       TEXT NOT NULL CHECK (char_length(label) <= 512),
  color       TEXT CHECK (color SIMILAR TO '#[0-9A-Fa-f]{8}'),
  sort_order  INTEGER NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One scoped row per (scope, purpose, value).
CREATE UNIQUE INDEX tag_templates_scoped_purpose_value_idx
  ON tag_templates (scope, purpose, value) WHERE scope IS NOT NULL;

-- One GLOBAL row per (purpose, value). A plain UNIQUE (scope, purpose, value)
-- would NOT dedupe global rows because NULL != NULL, so global uniqueness needs
-- its own partial index over the scope-IS-NULL subset.
CREATE UNIQUE INDEX tag_templates_global_purpose_value_idx
  ON tag_templates (purpose, value) WHERE scope IS NULL;

-- Supports the list query's (purpose [, scope]) filter.
CREATE INDEX tag_templates_purpose_scope_idx ON tag_templates (purpose, scope);

CREATE TRIGGER tag_templates_set_updated_at
  BEFORE UPDATE ON tag_templates
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS tag_templates;
```

Notes:
- `scope` FKs mod-core's `apps(id)` (an FK-anchor table keyed on `entities.id`;
  see `mod-core/model/migrations/0015_apps.sql`). This is the same
  cross-module-FK-within-one-composed-DB pattern the existing `tags` table already
  uses against `entities`; mod-tags declares `migrations.after: [core]`, so `apps`
  exists before `0204` runs at runtime. **Do not** add an `apps` migration to
  `model/migrations/` — mod-core owns it.
- `ON DELETE CASCADE` on `scope`: removing an app removes its scoped templates.
- `set_updated_at` is defined in the core helpers migration (already present at
  runtime and in the sqlc mirror).
- No seed rows. No `types` seed. No trigger wiring `tag_templates` into the
  entity/type machinery.

### 2. sqlc schema mirror — `model/schema/migrations/`

sqlc generates from `model/schema/migrations/` (per `model/sqlc.yaml`), a curated
mirror that currently lacks mod-core's `apps` table. The new FK will fail
`sqlc generate` unless `apps` is present in the mirror.

- Add mod-core's `apps` table definition to the mirror (copy
  `mod-core/model/migrations/0015_apps.sql`, e.g. as
  `model/schema/migrations/0015_apps.sql`). The mirror already contains the
  dependencies its trigger references (`type_is_or_descends_from` in `0002`), so a
  verbatim copy should parse. If sqlc rejects the `apps_check_type` trigger
  function or its `'app'`-type seed dependency, fall back to a **minimal
  table-only** `CREATE TABLE apps (id BIGINT PRIMARY KEY REFERENCES entities(id)
  ON DELETE RESTRICT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL);` in the
  mirror — the mirror exists only for sqlc type resolution, not runtime. If the
  full copy needs the `'app'` type seed, also mirror
  `mod-core/model/migrations/0014_type_app.sql`.
- Add the tag-templates schema to the mirror as
  `model/schema/migrations/0204_tag_templates.sql` (same DDL as the runtime
  migration; the `+goose` markers are harmless to sqlc but keep the file identical
  to the runtime one for parity).

### 3. sqlc query file — `model/queries/tag_templates.sql`

Add a new query file (do not modify `model/queries/tags.sql`). One list query
using the module's established `sqlc.narg` optional-filter idiom, returning the
scope's app UUID via a `LEFT JOIN entities`:

```sql
-- name: ListTagTemplates :many
SELECT tt.purpose, tt.value, tt.label, tt.color, tt.sort_order,
       tt.scope, e.uuid AS scope_uuid
FROM tag_templates tt
LEFT JOIN entities e ON e.id = tt.scope
WHERE tt.purpose = @purpose
  AND (tt.scope IS NULL OR tt.scope = sqlc.narg('scope')::bigint)
ORDER BY tt.scope NULLS FIRST, tt.sort_order ASC, tt.value ASC;
```

Behavior: a NULL `scope` narg returns only global rows (`scope IS NULL`); a set
`scope` narg returns globals **plus** that app's scoped rows. (This globals+scoped
behavior for the scope-set case is a planner assumption flagged for the manager;
if the owner wants scoped-only, tighten the predicate to
`tt.scope = sqlc.narg('scope')::bigint`.)

### 4. Regenerate — `model/db/`

Run `sqlc generate` (or `make gen`/`make build` in `model/`). Commit the
regenerated `model/db/tag_templates.sql.go`, and the updated `models.go` /
`querier.go`. Do not hand-edit generated files.

## Validation

- `cd model && make verify` (goose validate + sqlc compile) passes.
- `cd model && sqlc generate` produces `model/db/tag_templates.sql.go` and updates
  `models.go`/`querier.go`; `git status` shows only additive/generated changes,
  no edits under `tags`-related files.
- Shadow-DB lint passes: `cd model && make lint` (ephemeral Postgres applies all
  migrations, including `0204`, cleanly with no gaps).
- Confirm the migration applies forward and rolls back against a clean schema.
- Grep confirms the existing `tags` table DDL and `model/queries/tags.sql` are
  unchanged (`git diff --stat` lists only new files + regenerated `model/db`).
- Manual constraint check (in the shadow/lint DB or a test): two `scope IS NULL`
  rows with the same `(purpose, value)` are rejected by
  `tag_templates_global_purpose_value_idx`; two rows with the same
  `(scope, purpose, value)` for a non-null scope are rejected by
  `tag_templates_scoped_purpose_value_idx`.

## Metadata

architectural_impact: true

## References

- [Schema grounding notes](../notes/schema-grounding.md) — `apps` table, sqlc
  mirror gap, unique-index decision, all confirmed against actual schema.
- `mod-core/model/migrations/0015_apps.sql` — the `apps` FK-target definition to
  mirror.
- `model/migrations/0201_tags.sql` — the existing analogous subtype table
  (column types, CHECK constraints, `set_updated_at` trigger pattern to mirror).
- `model/queries/tags.sql` — the `sqlc.narg` optional-filter idiom.
- `model/sqlc.yaml` — confirms `schema: "./schema/migrations"` as the sqlc source.
- SQL Design Standards — partial/expression indexes, additive-migration safety,
  parameterized queries.

## Checkpoint hints

- After authoring the runtime migration `0204_tag_templates.sql`.
- After adding `apps` + `0204` to the sqlc schema mirror and getting
  `sqlc generate` to succeed.
- After adding the query file and committing the regenerated `model/db/`.

## Status

**Outcome:** succeeded. Date: 2026-07-18.

**Requirement #2 implemented differently than literally specified — flagged
decision, not a silent deviation.** The task doc (and the grounding notes it
cites) describe `model/schema/migrations/` as a "curated mirror" that needs
`apps` hand-added as a committed file, with a documented minimal-table
fallback if the full `mod-core/model/migrations/0015_apps.sql` copy trips
sqlc. Grounding against the actual repo state (not just the notes) shows this
directory is **not** a committed mirror at all:

- `model/.gitignore` has `/schema/` — the whole directory is gitignored.
- `model/Makefile`'s `compose` target (`rm -rf schema/migrations; mkdir -p
  schema/migrations; cp $(CORE_DIR)/migrations/*.sql schema/migrations/; cp
  migrations/*.sql schema/migrations/`) wholly regenerates the directory on
  every `make build`/`migrate.up`/`migrate.status` from mod-core's *real*
  `migrations/` dir plus this module's own `migrations/` dir — it is
  build-time output, confirmed by git history (commit `b27d16d`: "Align with
  users-module convention: schema/migrations/ is a build-time compose of
  core + own migrations, not checked in").
- mod-core's real `model/migrations/` already contains the full `apps` table
  definition (`0014_type_app.sql`, `0015_apps.sql`, `0016_apps_updated_at.sql`)
  — nothing needed to be hand-authored or mirrored; `make compose` copies it
  verbatim, and `sqlc compile`/`sqlc generate` resolved the `tag_templates.scope
  … REFERENCES apps(id)` FK cleanly against that real composition with zero
  hand-added files.

Given this, hand-authoring `model/schema/migrations/0015_apps.sql` (or a
`0204_tag_templates.sql` copy) would have been immediately destroyed by the
next `make compose`/`make build` invocation (the target does `rm -rf
schema/migrations` first) and could not be committed under the project's own
`.gitignore` convention without `git add -f`, which would fight that
convention on every future compose. So Requirement #2's *literal* file-adding
instruction was not followed; its *intent* (make `apps` resolvable to sqlc so
`tag_templates`'s FK compiles and generates) was fully satisfied via the
project's actual, already-established compose mechanism. **The grounding
notes' claim that the mirror "currently lacks `apps`" is stale** — true only
in the sense that the (gitignored, absent-until-built) directory has nothing
in it before `make compose` runs, but once composed (a mandatory step before
`sqlc generate`/`compile` can run at all — `schema/migrations` doesn't exist
otherwise) `apps` is present automatically, in full. Flagged for the manager
in case the grounding notes should be corrected for future tasks that read
them.

**Validation summary:**
- `sqlc compile` / `sqlc generate` (via `make compose
  CORE_DIR=<real-mod-core-path>` then `sqlc generate` in `model/`): passed.
  Produced `model/db/tag_templates.sql.go` and additive updates to
  `models.go` (new `App`, `TagTemplate` structs) / `querier.go` (new
  `ListTagTemplates` method). `git diff --stat` against the phase-start
  commit shows only new files (`0204_tag_templates.sql`,
  `queries/tag_templates.sql`, `db/tag_templates.sql.go`) plus additive
  `db/models.go` / `db/querier.go` changes; `model/db/tags.sql.go` and
  `model/queries/tags.sql` are untouched.
- `go build ./...` and `go vet ./...` in `model/`: passed.
- `cd model && make verify`: **failed**, but on a pre-existing,
  unrelated-to-this-task defect: `goose -dir migrations validate` errors on
  `migrations/migrate.go` ("no filename separator '_' found") because that
  directory also holds the module's non-SQL `migrate.go` helper (a
  `//go:embed *.sql` + `Migrate()` entrypoint). Reproduced the identical
  failure against the **unmodified** `mod-tags/model` checkout (i.e. before
  any change from this task), confirming it predates and is unrelated to
  `0204_tag_templates.sql`. The `sqlc compile` half of `make verify` passes
  on its own (see above). Not fixed — out of this task's scope (would mean
  editing `migrate.go` or the Makefile's `verify` target, neither named in
  `## Requirements`).
- `cd model && make lint` (shadow-db-lint.sh): **failed** in this execution
  environment on Docker container-IP connectivity
  (`dial tcp 172.17.0.2:5432: connect: operation timed out`), reproduced
  identically against the unmodified checkout — an environment/sandbox
  limitation of the script's direct-container-IP connection strategy, not a
  migration-content problem. Not fixed — modifying the shared
  `scripts/shadow-db-lint.sh` is out of this task's scope. **Substituted
  equivalent manual validation** using the same `goose` binary against an
  ephemeral `postgres:16` container reached via host port-forwarding instead
  of container IP: applied the full composed `schema/migrations` (mod-core
  0001–0016, 0099, mod-tags 0200–0204, 0299) forward with `goose up` —
  all 23 migrations including `0204_tag_templates.sql` applied cleanly, no
  gaps, ending at version 299. Rolled `0204_tag_templates.sql` back with
  `goose down` — table dropped cleanly — then re-applied with `goose up` —
  reapplied cleanly. Manually verified both partial unique indexes reject
  duplicates as designed: two `scope IS NULL` rows with the same
  `(purpose, value)` → rejected by `tag_templates_global_purpose_value_idx`;
  two rows with the same `(scope, purpose, value)` for a non-null scope →
  rejected by `tag_templates_scoped_purpose_value_idx`. Also exercised the
  `updated_at` trigger (fires on `UPDATE`), the `ON DELETE CASCADE` from
  `apps` to `tag_templates.scope` (deleting the owning `apps` row cascade-
  deleted its scoped `tag_templates` row), and the `ListTagTemplates` query's
  documented behavior directly in SQL (NULL `scope` → globals only; set
  `scope` → globals + that app's scoped rows, with `scope_uuid` correctly
  resolved via the `LEFT JOIN entities`).
- `git diff --stat` (existing `tags` files unchanged): confirmed — no edits
  under `model/migrations/02{00,01,02,03,99}_*.sql`, `model/queries/tags.sql`,
  or `model/db/tags.sql.go`.

**Assumptions applied (from the task doc's `## References` / inline notes,
not re-litigated here):** `scope … ON DELETE CASCADE` per the flagged
assumption; `label` `NOT NULL` per the flagged assumption; the
`ListTagTemplates` globals+scoped-on-set-scope behavior implemented exactly
as specified (flagged in the task doc itself as an assumption the owner may
want to tighten to scoped-only).

**Environment note (not a code change):** inside this task's provisioned
worktree, `model/Makefile`'s `CORE_DIR` fallback (`../../mod-core/model`,
used when `go list -m github.com/moduleforge/core-model` fails, which it
does here — `core-model` is not a Go module dependency of `tags-model`)
resolves relative to the worktree's nested path and does not reach the real
sibling `mod-core` checkout. All `make compose`/`sqlc`/`goose` invocations
above were run with an explicit `CORE_DIR=/Users/zane/playground/moduleforge/mod-core/model`
override to work around this. This is purely a worktree-path artifact, not a
defect: from the actual `mod-tags/model` checkout (a real sibling of
`mod-core/`), the fallback resolves correctly, so no Makefile change is
warranted.

**Files touched:**
- `model/migrations/0204_tag_templates.sql` (new)
- `model/queries/tag_templates.sql` (new)
- `model/db/tag_templates.sql.go` (new, generated)
- `model/db/models.go` (additive, generated)
- `model/db/querier.go` (additive, generated)
- `plan/phase-01-tag-templates-catalog/001-model-schema-and-queries.md` (this
  file — status only)
</content>
