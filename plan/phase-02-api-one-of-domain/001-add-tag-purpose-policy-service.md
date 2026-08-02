# Add Tag Purpose Policy Service

## Purpose and scope

Add `TagPurposePolicyServicer`/`TagPurposePolicyService` in a new
`api/service/tag_purpose_policy.go`, mirroring `TagTemplateServicer`/`TagTemplateService`
(`api/service/tag_template.go`)'s existing shape and conventions closely: an
internal/administrative capability with **no HTTP route** registered for it in this
plan (see `plan/overview.md`, Design decision 4's "Flag CRUD home"). **Depends on**
Phase 1 being complete (imports the regenerated `tagsdb.Querier` with
`GetTagPurposePolicy`/`UpsertTagPurposePolicy`). **Parallel-eligible** with this
phase's other task (`002-thread-one-of-domain-through-tag-api.md`) — they touch
different files.

## Requirements

### 1. Service type and interface

```go
package service

// TagPurposePolicy is the service-layer representation of a tag_purpose_policies row.
type TagPurposePolicy struct {
	Purpose      string
	OneOfDomain  bool
}

// TagPurposePolicyServicer defines access to the tag_purpose_policies registry: an
// internal Get (used by callers that need to check/display a specific purpose's
// policy) and an internal administrative Upsert. Neither is exposed via any HTTP
// route in this plan — one_of_domain is a curated, admin-set value, not end-user or
// public-API-writable; values are seeded/managed out-of-band (see
// plan/overview.md's Design decision 4 and next-steps.md). Kept as its own type
// (not folded into TagTemplateServicer) because tag_purpose_policies is a distinct
// table/concept from tag_templates — see plan/overview.md's "Naming note".
type TagPurposePolicyServicer interface {
	// Get returns the policy for purpose, or a zero-value TagPurposePolicy with
	// OneOfDomain: false when no row exists — the same default the DB trigger
	// applies (see model/migrations/0205_tags_one_of_domain.sql). Never an error
	// for the "no row" case; only for genuine query failures.
	Get(ctx context.Context, tagQ tagsdb.Querier, purpose string) (TagPurposePolicy, error)

	// Upsert inserts or updates the one_of_domain flag for purpose. Internal/
	// administrative only — mirrors TagTemplateServicer.Upsert's doc-commented
	// convention exactly; no route calls it.
	Upsert(ctx context.Context, tagQ tagsdb.Querier, purpose string, oneOfDomain bool) (TagPurposePolicy, error)
}
```

Follow `TagTemplateService`'s pattern: `TagPurposePolicyService struct{}` (no state,
zero value ready to use — this table has no per-row authorization, same as
`tag_templates`), a compile-time `var _ TagPurposePolicyServicer = (*TagPurposePolicyService)(nil)`
assertion, and doc comments on the struct explaining the no-Authorizer,
no-HTTP-route convention (copy `TagTemplateService`'s doc-comment style/wording where
it applies).

### 2. Implementation

- `Get`: trim `purpose`; if empty, `fmt.Errorf("%w: purpose is required", ErrInvalidInput)`.
  Call `tagQ.GetTagPurposePolicy(ctx, purpose)`; on `errors.Is(err, pgx.ErrNoRows)`,
  return `TagPurposePolicy{Purpose: purpose, OneOfDomain: false}, nil` (not an error —
  matches the DB trigger's own default-false-when-absent behavior). Any other error:
  wrap as `fmt.Errorf("tagpurposepolicy.Get query: %w", err)`.
- `Upsert`: trim `purpose`; validate non-empty the same way. Call
  `tagQ.UpsertTagPurposePolicy(ctx, tagsdb.UpsertTagPurposePolicyParams{Purpose: purpose, OneOfDomain: oneOfDomain})`
  and hydrate the result. Wrap query errors similarly (`tagpurposepolicy.Upsert query: %w`).

### 3. Wire into `Services` aggregate

In `api/service/service.go`, add a `TagPurposePolicy TagPurposePolicyServicer` field to
the `Services` struct (alongside the existing `Tag`/`TagTemplate` fields) with a doc
comment mirroring `TagTemplate`'s ("has no dependencies of its own... kept separate so
the tags CRUD paths are unaffected"), and construct it in `New()`:
`TagPurposePolicy: &TagPurposePolicyService{}`. **Do not** add anything to
`api/httpapi/` in this task — no route, no handler. That is a deliberate scope
boundary; do not add one "for completeness."

### 4. Tests

New `api/service/tag_purpose_policy_test.go`, following `tag_template_test.go`'s
structure and the existing `mockTagQuerier` (`api/service/mock_test.go`). You will need
to extend `mockTagQuerier` with a `policies map[string]bool` field plus
`GetTagPurposePolicy`/`UpsertTagPurposePolicy` mock methods (and a `seedPurposePolicy`
helper) to satisfy the now-larger `tagsdb.Querier` interface — this mirrors how
`upsertedTemplates`/`seedTemplate` already work for `tag_templates`. Coordinate with
task `002-thread-one-of-domain-through-tag-api.md`'s mock-querier changes if working in
parallel (both tasks touch `mock_test.go`; keep each addition additive/independent —
new fields and methods, not edits to unrelated existing ones — to minimize merge
friction). Cover:
- `Get` on an unset purpose returns `{Purpose: p, OneOfDomain: false}`, no error.
- `Get`/`Upsert` reject empty purpose with `ErrInvalidInput`.
- `Upsert` then `Get` round-trips the flag; a second `Upsert` with a different value
  updates in place (not a duplicate row) — mirrors
  `tag_template_upsert_integration_test.go`'s idempotency-of-upsert intent, but this
  can stay a mock-backed unit test (no live-DB integration test is required for this
  task; task 002 owns the live-DB integration coverage for the trigger itself).

## Validation

- `cd api && go build ./...` and `go test ./...` pass.
- `cd api && make lint` (`go vet` + `gofmt` check) passes.
- `grep -n "TagPurposePolicy" api/service/service.go` shows the new field and
  constructor wiring.
- `grep -rn "TagPurposePolicy" api/httpapi/` returns **no** matches — confirms no route
  was added (the deliberate scope boundary from Requirement 3).
- New tests in `api/service/tag_purpose_policy_test.go` pass and cover the four
  bullets above.

## References

- `plan/overview.md` — Design decision 4's "Flag CRUD home" subsection.
- `api/service/tag_template.go` — the `TagTemplateServicer`/`TagTemplateService`
  pattern this task mirrors, including its "internal/administrative capability... no
  route calls it" doc-comment convention.
- `api/service/tag_template_test.go`, `api/service/tag_template_upsert_integration_test.go`
  — sibling test structure/precedent.
- `api/service/mock_test.go` — `mockTagQuerier`, `upsertedTemplates`/`seedTemplate` as
  the precedent for the new `policies`/`seedPurposePolicy` addition.
- Phase 1 task `002-expose-one-of-domain-in-tag-queries.md` — defines
  `GetTagPurposePolicy`/`UpsertTagPurposePolicy` this task calls.
