-- name: ListTagTemplates :many
SELECT tt.purpose, tt.value, tt.label, tt.color, tt.sort_order,
       tt.scope, e.uuid AS scope_uuid,
       COALESCE(tpp.one_of_domain, false) AS one_of_domain
FROM tag_templates tt
LEFT JOIN entities e ON e.id = tt.scope
LEFT JOIN tag_purpose_policies tpp ON tpp.purpose = tt.purpose
WHERE tt.purpose = @purpose
  AND (tt.scope IS NULL OR tt.scope = sqlc.narg('scope')::bigint)
ORDER BY tt.scope NULLS FIRST, tt.sort_order ASC, tt.value ASC;

-- UpsertTagTemplate inserts or updates a single app-scoped (non-null-scope)
-- tag_templates row, keyed by (scope, purpose, value). The ON CONFLICT
-- target repeats the "WHERE scope IS NOT NULL" predicate of the
-- tag_templates_scoped_purpose_value_idx partial unique index (see
-- 0204_tag_templates.sql) — Postgres requires an ON CONFLICT clause's
-- predicate to match a partial index's predicate exactly for it to be used
-- as the arbiter. This query is not usable for global (NULL-scope) rows,
-- which are deduped by a separate partial index instead.
-- name: UpsertTagTemplate :one
INSERT INTO tag_templates (scope, purpose, value, label, color, sort_order)
VALUES (sqlc.narg('scope')::bigint, @purpose, @value, @label, sqlc.narg('color'), @sort_order)
ON CONFLICT (scope, purpose, value) WHERE scope IS NOT NULL
DO UPDATE SET label = EXCLUDED.label, color = EXCLUDED.color,
              sort_order = EXCLUDED.sort_order, updated_at = now()
RETURNING id, scope, purpose, value, label, color, sort_order;
