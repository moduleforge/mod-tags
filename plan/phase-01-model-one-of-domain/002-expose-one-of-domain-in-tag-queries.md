# Expose One Of Domain In Tag Queries

## Purpose and scope

Extend the six existing tag read/write queries in `model/queries/tags.sql` so each
returns a computed `one_of_domain` column (`false` when the tag's `purpose` has no
`tag_purpose_policies` row), so Phase 2 can thread it into every tag-returning API
response (see `plan/overview.md`, Design decision 4). **Depends on task
`001-add-tag-purpose-policies-table.md`** — the `tag_purpose_policies` table must
already exist.

## Requirements

Modify exactly these six named queries in `model/queries/tags.sql` — no others.
(`GetTagByEntityID` is deliberately **not** touched: it is used only by
`api/service/display.go`'s display-name renderer, which never returns a full `Tag` to
an API response, so it does not need `one_of_domain`.)

### 1. `CreateTag` (INSERT ... RETURNING) — add a scalar subquery

```sql
-- name: CreateTag :one
INSERT INTO tags (entity_id, owner_id, subject_id, purpose, value, color)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING entity_id, owner_id, subject_id, purpose, value, color, created_at, updated_at,
  COALESCE((SELECT one_of_domain FROM tag_purpose_policies WHERE purpose = tags.purpose), false) AS one_of_domain;
```

### 2. `UpdateTagColor` and `UpdateTagValue` (UPDATE ... RETURNING) — same subquery pattern

```sql
-- name: UpdateTagColor :one
UPDATE tags
SET color = @color
WHERE entity_id = @entity_id
RETURNING entity_id, owner_id, subject_id, purpose, value, color, created_at, updated_at,
  COALESCE((SELECT one_of_domain FROM tag_purpose_policies WHERE purpose = tags.purpose), false) AS one_of_domain;

-- name: UpdateTagValue :one
UPDATE tags
SET value = @value
WHERE entity_id = @entity_id
RETURNING entity_id, owner_id, subject_id, purpose, value, color, created_at, updated_at,
  COALESCE((SELECT one_of_domain FROM tag_purpose_policies WHERE purpose = tags.purpose), false) AS one_of_domain;
```

### 3. `GetTagByEntityUUID`, `ListTagsBySubjectEntityID`, `SearchTags` (SELECT ... JOIN) — `LEFT JOIN`

Add `LEFT JOIN tag_purpose_policies tpp ON tpp.purpose = t.purpose` alongside each
query's existing `JOIN entities e ...`, and add
`COALESCE(tpp.one_of_domain, false) AS one_of_domain` to the `SELECT` list. Example for
`GetTagByEntityUUID`:

```sql
-- name: GetTagByEntityUUID :one
SELECT t.entity_id, t.owner_id, t.subject_id, t.purpose, t.value, t.color,
       t.created_at, t.updated_at, e.uuid,
       COALESCE(tpp.one_of_domain, false) AS one_of_domain
FROM tags t
JOIN entities e ON e.id = t.entity_id
LEFT JOIN tag_purpose_policies tpp ON tpp.purpose = t.purpose
WHERE e.uuid = $1;
```

Apply the same `LEFT JOIN tag_purpose_policies tpp ON tpp.purpose = t.purpose` +
`COALESCE(tpp.one_of_domain, false) AS one_of_domain` shape to `ListTagsBySubjectEntityID`
and `SearchTags` (both already `JOIN entities e` and
`JOIN accessible_tag_ids_for_actor(...) acc`; add the new `LEFT JOIN` alongside those,
and the new column to each `SELECT` list, preserving existing column order and adding
`one_of_domain` last).

### 4. Regenerate sqlc

Run `cd model && sqlc generate`. This changes the return type of `CreateTag`,
`UpdateTagColor`, and `UpdateTagValue` from the bare `Tag` struct to new
`CreateTagRow`/`UpdateTagColorRow`/`UpdateTagValueRow` types (since their `RETURNING`
column set no longer exactly matches the `tags` table) — **this is expected and is
exactly what Phase 2 task 002 needs**; flag it explicitly in your task report so the
Phase 2 implementer isn't surprised, but do not attempt to preserve the old `Tag`
return type (e.g. by casting or omitting the new column from `RETURNING`) — that would
defeat the purpose of this task. `GetTagByEntityUUIDRow`, `ListTagsBySubjectEntityIDRow`,
and `SearchTagsRow` simply grow a new `OneOfDomain bool` field; their type names are
unchanged.

Commit the regenerated `model/db/tags.sql.go` and any changed `model/db/querier.go`.

## Validation

- `cd model && make verify` (`goose validate` + `sqlc compile`) passes.
- `cd model && make lint` (ephemeral-Postgres shadow-DB migration apply) still passes —
  this task does not add a migration, but re-running lint after `sqlc generate` is a
  cheap sanity check that nothing regressed.
- `grep -n "one_of_domain" model/queries/tags.sql` shows exactly six occurrences (one
  per named query touched).
- `grep -n "OneOfDomain" model/db/tags.sql.go` shows the field present on
  `CreateTagRow`, `GetTagByEntityUUIDRow`, `ListTagsBySubjectEntityIDRow`,
  `SearchTagsRow`, `UpdateTagColorRow`, `UpdateTagValueRow` — confirm via `grep -B5
  "OneOfDomain bool"` that each of those six struct names appears, and that the plain
  `Tag` struct (from `GetTagByEntityID`) does **not** grow the field (it should still
  match exactly the `tags` table's columns).
- `model/db/querier.go`'s `Querier` interface: confirm `CreateTag` now returns
  `(CreateTagRow, error)`, `UpdateTagColor` returns `(UpdateTagColorRow, error)`,
  `UpdateTagValue` returns `(UpdateTagValueRow, error)` — these signature changes are
  the load-bearing input Phase 2 task 002 depends on; do not leave them as `(Tag,
  error)`.
- A quick manual sanity check (either via `make -C model test.integration` against a
  local DB, or by reading the generated SQL text in `model/db/tags.sql.go`) that the
  `COALESCE(...)` / `LEFT JOIN` additions are syntactically well-formed — this task
  does not need new integration tests of its own (Phase 2 task 002 adds the
  service-level integration test that exercises these queries end-to-end through a
  real trigger conflict).

## Metadata

architectural_impact: true

## References

- `plan/overview.md` — Design decision 4 ("Exposing 'occupied' purposes to the GUI").
- `model/queries/tags.sql`, `model/db/tags.sql.go`, `model/db/querier.go` — current
  state of the six queries and the generated interface being extended.
- Task `001-add-tag-purpose-policies-table.md` — the table/trigger this task's queries
  join against.
