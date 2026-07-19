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
</content>
