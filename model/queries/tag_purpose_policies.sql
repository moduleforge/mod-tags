-- name: GetTagPurposePolicy :one
SELECT purpose, one_of_domain, created_at, updated_at
FROM tag_purpose_policies
WHERE purpose = $1;

-- name: UpsertTagPurposePolicy :one
INSERT INTO tag_purpose_policies (purpose, one_of_domain)
VALUES (@purpose, @one_of_domain)
ON CONFLICT (purpose) DO UPDATE SET one_of_domain = EXCLUDED.one_of_domain, updated_at = now()
RETURNING purpose, one_of_domain, created_at, updated_at;
