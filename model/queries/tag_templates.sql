-- name: ListTagTemplates :many
SELECT tt.purpose, tt.value, tt.label, tt.color, tt.sort_order,
       tt.scope, e.uuid AS scope_uuid
FROM tag_templates tt
LEFT JOIN entities e ON e.id = tt.scope
WHERE tt.purpose = @purpose
  AND (tt.scope IS NULL OR tt.scope = sqlc.narg('scope')::bigint)
ORDER BY tt.scope NULLS FIRST, tt.sort_order ASC, tt.value ASC;
