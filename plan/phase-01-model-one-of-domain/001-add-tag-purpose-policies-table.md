# Add Tag Purpose Policies Table And Enforcement Trigger

## Purpose and scope

Add the new `tag_purpose_policies` table (the per-`purpose` `one_of_domain` policy
registry — see `plan/overview.md`'s Design decision 1) and the trigger-based
enforcement mechanism that replaces `tags`' current unconditional
`tags_owner_subject_purpose_idx` unique index with a conditional, policy-driven check
(Design decision 3). This is a standard goose migration + sqlc query addition; no named
skill needed beyond following this module's existing migration conventions (see
`AGENTS.md`'s "Database migrations" and "Code generation (sqlc)" sections, and
`docs/mf-standards/architecture/db-considerations.md`'s "Migration file conventions
under goose").

## Requirements

### 1. New migration: `model/migrations/0205_tags_one_of_domain.sql`

`0204_tag_templates.sql` is the current highest "real" migration; `0299` is a reserved,
always-last override slot (see `model/README.md`, `moduleforge.module.yaml`'s
`migrations.range`) — do not renumber or touch it. Use `0205`.

The migration has three parts, all under a single `-- +goose Up` (no `-- +goose Down`
section — this module's migrations are forward-only by convention; see `AGENTS.md`'s
"Database migrations" section. `0204_tag_templates.sql`'s `-- +goose Down` section is a
pre-existing exception, not a pattern to follow here):

**a. `tag_purpose_policies` table:**

```sql
CREATE TABLE tag_purpose_policies (
  purpose        TEXT NOT NULL PRIMARY KEY CHECK (char_length(purpose) <= 512),
  one_of_domain  BOOLEAN NOT NULL DEFAULT false,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER tag_purpose_policies_set_updated_at
  BEFORE UPDATE ON tag_purpose_policies
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

`set_updated_at()` is already defined (used by `tags_set_updated_at` and
`tag_templates_set_updated_at` — it is a shared core-provided function; do not
redefine it). Add a short comment above the table (mirroring `0204_tag_templates.sql`'s
style) explaining: this table is keyed purely on `purpose` (not `scope`); it is
distinct from `tag_templates` (open/read-only suggestion catalog, no auth gating) —
this table backs enforcement; absence of a row for a purpose means
`one_of_domain = false` (the default — see part c below); administered out-of-band, no
public write endpoint in this plan (Phase 2 adds an internal-only servicer).

**b. Convert the existing unique index to non-unique:**

```sql
DROP INDEX tags_owner_subject_purpose_idx;
CREATE INDEX tags_owner_subject_purpose_idx
  ON tags (owner_id, subject_id, purpose);
```

Keep the same index name and column list — only the uniqueness is removed. Add a
comment explaining why (conditional uniqueness driven by a different table cannot be
expressed as a plain/partial index; enforcement moves to the trigger in part c; the
index itself is retained for lookup performance).

**c. Enforcement trigger, `tags_enforce_one_of_domain`:**

```sql
-- +goose StatementBegin
CREATE FUNCTION tags_enforce_one_of_domain() RETURNS TRIGGER AS $$
DECLARE
  v_one_of_domain BOOLEAN;
BEGIN
  SELECT one_of_domain INTO v_one_of_domain
  FROM tag_purpose_policies
  WHERE purpose = NEW.purpose;

  IF NOT FOUND THEN
    v_one_of_domain := false;
  END IF;

  IF v_one_of_domain THEN
    PERFORM pg_advisory_xact_lock(
      hashtextextended(NEW.owner_id::text || ':' || NEW.subject_id::text || ':' || NEW.purpose, 0)
    );

    IF EXISTS (
      SELECT 1 FROM tags
      WHERE owner_id = NEW.owner_id
        AND subject_id = NEW.subject_id
        AND purpose = NEW.purpose
    ) THEN
      RAISE EXCEPTION 'tags: purpose % is one-of-domain; owner % already has a tag of this purpose on subject %',
        NEW.purpose, NEW.owner_id, NEW.subject_id
        USING ERRCODE = 'unique_violation';
    END IF;
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER tags_enforce_one_of_domain
  BEFORE INSERT ON tags
  FOR EACH ROW EXECUTE FUNCTION tags_enforce_one_of_domain();
```

Key points to preserve (do not simplify away — see `plan/overview.md` Design decision 3
for full rationale):
- **`BEFORE INSERT` only.** `purpose`/`owner_id`/`subject_id` are already immutable
  after insert (`tags_reject_immutable_changes`), so no `UPDATE` path can create a new
  conflict.
- **`USING ERRCODE = 'unique_violation'`** is deliberate — it reuses SQLSTATE `23505`
  so `api/service/tag.go`'s existing `pgErr.Code == pgUniqueViolation` check (in
  `TagService.Create`) already classifies this as `ErrConflict` (409) with zero Go
  changes. Do not use a different/custom SQLSTATE.
- **`pg_advisory_xact_lock`** closes the check-then-insert race a trigger-based check
  has relative to a true unique index. Do not drop this call as "unnecessary" — it is
  the race-safety mitigation called out explicitly in the design.
- Trigger name `tags_enforce_one_of_domain` must sort alphabetically after
  `tags_check_type` (so entity-type validation still runs first on the same `INSERT`)
  — this is already satisfied by the chosen name (`c` < `e`); do not rename.

### 2. New query file: `model/queries/tag_purpose_policies.sql`

```sql
-- name: GetTagPurposePolicy :one
SELECT purpose, one_of_domain, created_at, updated_at
FROM tag_purpose_policies
WHERE purpose = $1;

-- name: UpsertTagPurposePolicy :one
INSERT INTO tag_purpose_policies (purpose, one_of_domain)
VALUES (@purpose, @one_of_domain)
ON CONFLICT (purpose) DO UPDATE SET one_of_domain = EXCLUDED.one_of_domain, updated_at = now()
RETURNING purpose, one_of_domain, created_at, updated_at;
```

`GetTagPurposePolicy` returns `pgx.ErrNoRows` when no policy row exists for a purpose
— callers (Phase 2) treat that as `one_of_domain = false`, matching the trigger's own
`NOT FOUND` fallback.

### 3. Regenerate sqlc

Run `cd model && sqlc generate` (or `make -C model build`) after adding the query file.
Commit the regenerated `model/db/*.go` files — this module commits generated code (see
`AGENTS.md`: "Generated code (`model/db/`) is committed and should not be edited by
hand").

### 4. `model/README.md` touch-up

Add `tag_purpose_policies` to the opening paragraph's table list, alongside the
existing mention of `tag_templates` (currently: "the tag hierarchy: `tags`,
`entity_tags`, and related tables — plus the `tag_templates` catalog table, a plain
(non-entity) table of suggested purpose/value/label/color combinations"). One clause is
enough — e.g. "...plus the `tag_templates` suggestion catalog and the
`tag_purpose_policies` per-purpose policy registry." Do not otherwise rewrite this
README; the fuller decision-record writeup belongs to Phase 4.

## Validation

- `cd model && goose -dir migrations validate` (or `make -C model verify`, which runs
  `goose validate` + `sqlc compile` per `model/README.md`'s Make targets) passes.
- `cd model && make lint` (ephemeral-Postgres shadow-DB apply of every migration, per
  `model/scripts/shadow-db-lint.sh`) passes — this is the concrete proof the new
  migration applies cleanly on top of `0200`–`0204` and that the
  `pg_advisory_xact_lock`/`hashtextextended` calls are valid Postgres syntax.
- `sqlc generate` produces no errors; `git diff --stat` shows the new
  `model/db/tag_purpose_policies.sql.go` file and an updated `model/db/querier.go`
  (adding `GetTagPurposePolicy`/`UpsertTagPurposePolicy` to the `Querier` interface).
  `tags.sql.go` should be **unchanged** by this task (query text there is untouched
  until task 002).
- `grep -n "tags_owner_subject_purpose_idx" model/migrations/0205_tags_one_of_domain.sql`
  shows both the `DROP INDEX` and the replacement non-unique `CREATE INDEX`.
- `grep -n "unique_violation" model/migrations/0205_tags_one_of_domain.sql` confirms
  the `USING ERRCODE` clause is present.
- Manual read-through: confirm the trigger fires `BEFORE INSERT` only (not `BEFORE
  UPDATE`), and that `tags_enforce_one_of_domain` is not accidentally misspelled
  relative to `tags_check_type` in a way that would break the alphabetical firing
  order.
- `model/README.md` mentions `tag_purpose_policies`.

## Metadata

architectural_impact: true

## References

- `plan/overview.md` — Design decisions 1–3 (full rationale for the table shape, the
  default, and the trigger mechanism).
- `model/migrations/0201_tags.sql` — the `tags_check_type()` / `tags_reject_immutable_changes()`
  trigger style this migration mirrors.
- `model/migrations/0204_tag_templates.sql` — sibling small-table migration for
  structural/comment-style precedent.
- `docs/decisions/tags-limited-immutability.md` — the established immutability-trigger
  pattern this module already documents.
- `AGENTS.md` — "Database migrations" and "Code generation (sqlc)" sections.
- `docs/mf-standards/architecture/db-considerations.md` — "Migration file conventions
  under goose".
