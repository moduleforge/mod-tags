# Thread One Of Domain Through Tag Api

## Purpose and scope

Thread the new `one_of_domain` column (added to six queries by Phase 1 task
`002-expose-one-of-domain-in-tag-queries.md`) through `service.Tag`, the
`TagService` hydration helpers, and `httpapi`'s response shaping, so every
tag-returning endpoint's JSON body carries `oneOfDomain: boolean`. Also proves the
create-conflict error path (Design decision 3/4's SQLSTATE-reuse trick) actually
classifies as `409 conflict` end-to-end. **Depends on** Phase 1 being complete.
**Parallel-eligible** with this phase's other task
(`001-add-tag-purpose-policy-service.md`).

## Requirements

### 1. `service.Tag` gains `OneOfDomain bool`

In `api/service/tag.go`, add `OneOfDomain bool` to the `Tag` struct (alongside
`Color *string`, etc.).

### 2. Fix up `hydrateTag`'s signature — this is the trickiest part

Phase 1 task 002 changed `CreateTag`, `UpdateTagColor`, and `UpdateTagValue`'s return
types from the bare `tagsdb.Tag` to new `tagsdb.CreateTagRow`,
`tagsdb.UpdateTagColorRow`, `tagsdb.UpdateTagValueRow` types (each has the same fields
as `tagsdb.Tag` plus `OneOfDomain bool`). `hydrateTag(entityUUID, ownerUUID,
subjectUUID uuid.UUID, t tagsdb.Tag) Tag` currently takes a single `tagsdb.Tag` — since
there are now three *different* generated struct types (plus the original `tagsdb.Tag`
via `tagFromUUIDRow`) that need to flow through it, **change `hydrateTag`'s signature
to take an explicit `oneOfDomain bool` parameter** rather than trying to keep a single
polymorphic row-type parameter:

```go
func hydrateTag(entityUUID, ownerUUID, subjectUUID uuid.UUID, t tagsdb.Tag, oneOfDomain bool) Tag {
	tag := Tag{
		EntityUUID:  entityUUID,
		OwnerUUID:   ownerUUID,
		SubjectUUID: subjectUUID,
		Purpose:     t.Purpose,
		Value:       t.Value,
		OneOfDomain: oneOfDomain,
	}
	// ...unchanged Color/CreatedAt/UpdatedAt logic below...
}
```

Update every call site to pass the new argument, reading `OneOfDomain` from whichever
row the call site actually has in hand:

- **`Create`** (in `tag.go`): `tag, err := txTagQ.CreateTag(...)` now returns
  `tagsdb.CreateTagRow`. Update the local `var tag ...` type accordingly (or let `:=`
  infer it) and change the call to
  `hydrateTag(entity.Uuid, ownerEntity.Uuid, in.SubjectEntityUUID, tagsdb.Tag{...fields from tag...}, tag.OneOfDomain)`
  — or, simpler: since `CreateTagRow` has the exact same non-`OneOfDomain` fields as
  `tagsdb.Tag`, construct a small local `tagsdb.Tag{EntityID: tag.EntityID, OwnerID:
  tag.OwnerID, SubjectID: tag.SubjectID, Purpose: tag.Purpose, Value: tag.Value, Color:
  tag.Color, CreatedAt: tag.CreatedAt, UpdatedAt: tag.UpdatedAt}` inline, or add a tiny
  `tagFromCreateRow(r tagsdb.CreateTagRow) tagsdb.Tag` helper mirroring the existing
  `tagFromUUIDRow` helper's style. Either is fine; prefer whichever keeps the diff
  smallest and matches this file's existing helper-function idiom (there are already
  three `hydrateTagFrom*`/`tagFromUUIDRow`-style helpers here — follow that pattern).
- **`GetByUUID`**: `row, err := tagQ.GetTagByEntityUUID(...)` (now carries
  `row.OneOfDomain`). Change
  `hydrateTag(row.Uuid, ownerEntity.Uuid, subjectEntity.Uuid, tagFromUUIDRow(row))` to
  `hydrateTag(row.Uuid, ownerEntity.Uuid, subjectEntity.Uuid, tagFromUUIDRow(row), row.OneOfDomain)`.
  This is the fix for the bug this task must not reintroduce: **`GetByUUID`'s response
  is API-visible and was at risk of silently dropping `OneOfDomain` via the
  `tagFromUUIDRow` round-trip** — do not skip this call site.
- **`UpdateByUUID`**: the `row` fetched via `txTagQ.GetTagByEntityUUID` (pre-update) is
  used only for the entity ID / before-snapshot, not for hydration. The final
  `hydrateTag(row.Uuid, ownerEntity.Uuid, subjectEntity.Uuid, updated)` call uses
  `updated` (result of `txTagQ.UpdateTagColor(...)`, now `tagsdb.UpdateTagColorRow`) —
  pass `updated.OneOfDomain` as the new argument.
- **`UpdateTagValue`**: same shape as `UpdateByUUID`, using `updated.OneOfDomain` from
  `txTagQ.UpdateTagValue(...)`'s `tagsdb.UpdateTagValueRow` result.
- **`DeleteByUUID`**: `tagFromUUIDRow(row)` feeds `hydrateTag` only to build an
  observer "before" snapshot (`tagSnapshot`), never an API response — pass
  `row.OneOfDomain` for signature compatibility, but see Requirement 4 below: **do
  not** add `one_of_domain` to `tagSnapshot`'s map.

- **`Search`**: uses `hydrateTagFromSearchRow` (a separate helper, not `hydrateTag`) —
  add `OneOfDomain: r.OneOfDomain` there directly (the row already carries it after
  Phase 1 task 002).
- **`ListBySubject`**: uses `hydrateTagFromListRow` — same treatment, add
  `OneOfDomain: r.OneOfDomain`.

### 3. `tagSnapshot` — deliberately excluded

`tagSnapshot(t Tag) map[string]any` (used for audit-log before/after observer payloads)
should **not** gain a `one_of_domain` key. `OneOfDomain` is a derived/policy attribute
borrowed from `tag_purpose_policies`, not part of the tag row's own persisted state —
audit snapshots should reflect only the tag's own fields, unchanged from today. This is
a deliberate decision, not an oversight; do not add it even though the field exists on
`Tag`.

### 4. `httpapi` response shape

In `api/httpapi/tags.go`:
- Add `OneOfDomain bool \`json:"oneOfDomain"\`` to `tagResponse`.
- Add `OneOfDomain: t.OneOfDomain` to `toTagResponse`.

No other `httpapi` file needs a code change — `subject_tags.go`'s
`handleSubjectTags` already calls the shared `toTagResponse`.

### 5. Update `mockTagQuerier` (and any other test doubles)

`api/service/mock_test.go`'s `mockTagQuerier.CreateTag`/`UpdateTagColor`/`UpdateTagValue`/
`GetTagByEntityUUID`/`ListTagsBySubjectEntityID`/`SearchTags` methods must be updated to
satisfy the new `tagsdb.Querier` interface signatures (return the new `*Row` types
where applicable, and populate `OneOfDomain`). Coordinate with task
`001-add-tag-purpose-policy-service.md` if run in parallel — that task also touches
`mock_test.go`, adding a `policies map[string]bool` field; **this task should read from
that same map** (do not invent a second, separate way to track per-purpose policy in
the mock) so `mockTagQuerier.CreateTag` etc. can compute `OneOfDomain` for the row they
return by looking up `m.policies[purpose]` (default `false` when absent — same
semantics as the real trigger/query).

Also extend `mockTagQuerier.CreateTag` to **simulate the one-of-domain conflict**: when
`m.policies[arg.Purpose]` is `true` and a tag already exists in `m.tags` with the same
`(OwnerID, SubjectID, Purpose)`, return a `*pgconn.PgError{Code: pgUniqueViolation}`
(the same constant already defined in `tag.go`) instead of inserting — this is what
lets a mock-backed unit test exercise the real Go-level classification path
(`TagService.Create`'s existing `errors.As(err, &pgErr) && pgErr.Code ==
pgUniqueViolation` check) without a live Postgres connection.

### 6. New tests

- **Unit test** (`api/service/tag_test.go` or a new file): seed the mock with a policy
  `{purpose: "priority", oneOfDomain: true}` and an existing tag
  `(owner=O, subject=S, purpose="priority", value="low")`; call
  `TagService.Create` with `(owner=O, subject=S, purpose="priority", value="urgent")`
  and assert `errors.Is(err, ErrConflict)`. Add a companion test proving a *different*
  purpose (or a purpose with no policy row / `oneOfDomain: false`) with the same
  owner/subject succeeds even when one already exists — proving the mock's simulated
  gate is conditional, not blanket.
- **Integration test** (new `//go:build integration` file, e.g.
  `api/service/tag_one_of_domain_integration_test.go`), following
  `tag_grant_integration_test.go`'s existing conventions (ephemeral composed-migrations
  directory, real Postgres, `TestMain` setup/teardown — reuse its established
  connectivity-probing approach rather than reinventing one): seed a
  `tag_purpose_policies` row with `one_of_domain = true` for a test purpose, insert one
  tag for `(owner, subject, purpose)` directly through `TagService.Create`, then assert
  a second `Create` call for the same `(owner, subject, purpose)` (different value)
  returns `ErrConflict` — proving the **real** DB trigger (not the mock's simulation)
  raises `SQLSTATE 23505` and that the existing Go-level classification picks it up
  end-to-end, with zero Go code changes to the classification logic itself (per Design
  decision 3). Run with `go test -tags=integration -p 1 -v ./service/...` from `api/`,
  matching the existing integration test's documented invocation.

## Validation

- `cd api && go build ./...` and `go test ./...` pass.
- `cd api && make lint` passes.
- `cd api && go test -tags=integration -p 1 -v ./service/...` passes (requires Docker;
  same environment precondition as the existing `tag_grant_integration_test.go`).
- `grep -n "OneOfDomain" api/service/tag.go` shows the field on `Tag` and its use in
  every hydrate call site enumerated in Requirement 2 (six call sites total: Create,
  GetByUUID, Search, ListBySubject, UpdateByUUID, UpdateTagValue; DeleteByUUID passes
  it through for signature compatibility only).
- `grep -n "one_of_domain" api/service/tag.go` (lowercase, snake_case) returns **no**
  matches inside `tagSnapshot` — confirms Requirement 3's deliberate exclusion.
- `grep -n "oneOfDomain" api/httpapi/tags.go` shows the new JSON field on
  `tagResponse` and its assignment in `toTagResponse`.
- New unit tests (Requirement 6) and the new integration test both pass.
- Existing tests (`tag_test.go`, `handlers_test.go`, `tag_template_test.go`, etc.)
  still pass unmodified except where the new `Tag`/`tagResponse` field or the changed
  mock-querier signatures require mechanical updates.

## Status

- **Outcome:** succeeded
- **Date:** 2026-08-02
- **Validation:**
  - `cd api && go build ./...` and `go test ./...` — **passed**. The three
    pre-existing `hydrateTag` signature-mismatch build errors task 001's own
    Status note flagged as expected/deferred are fixed by this task's
    Requirement 2 work; the whole package (and `httpapi`) now builds, vets,
    and tests green with zero remaining errors.
  - `cd api && make lint` — **passed** (`go vet ./...` clean; `gofmt -l .`
    empty).
  - `cd api && go test -tags=integration -p 1 -v ./service/...` — **passed**
    (33/33 tests, including the new `TestTagOneOfDomainIntegration`), against
    a live `users-module-postgres` Postgres container. This sandbox has a
    local Homebrew `postgresql@14` service permanently bound to
    `127.0.0.1:5432`/`[::1]:5432`, which shadows the Docker container's own
    `0.0.0.0:5432`-published port for both of `resolvePostgresHost`'s
    `localhost` and docker-network-IP candidates (the latter also times out,
    per that function's own Docker-Desktop-for-macOS doc comment) — this
    reproduces identically for the pre-existing, already-merged
    `tag_grant_integration_test.go`'s `TestTagGrantIntegration` when run
    standalone, confirming it is a sandbox-local port conflict, not a
    regression from this task. Worked around non-destructively (did not stop
    the user's native Postgres service) by setting
    `TAGS_DEV_PG_HOST=<host's LAN IP>` — a third candidate host reachable
    over TCP that the native Postgres service isn't bound to — which the
    harness's existing `TAGS_DEV_PG_HOST` override already supports with zero
    code changes.
  - `grep -n "OneOfDomain" api/service/tag.go` — **passed**: shows the field
    on `Tag` and its use at all six call sites (Create, GetByUUID, Search,
    ListBySubject, UpdateByUUID, UpdateTagValue; DeleteByUUID passes it
    through for signature compatibility only).
  - `grep -n "one_of_domain" api/service/tag.go` — the two lowercase
    snake_case occurrences are both in doc comments (referencing the
    migration file and the generated struct field name); neither is inside
    `tagSnapshot`, confirming Requirement 3's deliberate exclusion.
  - `grep -n "oneOfDomain" api/httpapi/tags.go` — **passed**: shows the field
    on `tagResponse` and its assignment in `toTagResponse`.
  - New unit tests
    (`TestTagService_Create_OneOfDomainConflict`,
    `TestTagService_Create_OneOfDomainConflict_DifferentPurposeSucceeds` in
    `tag_test.go`) and the new integration test
    (`TestTagOneOfDomainIntegration` in the new
    `tag_one_of_domain_integration_test.go`) all **passed**.
  - Existing tests — **passed unmodified**: no mechanical updates were
    required to any existing test file; `mockTagQuerier` (`mock_test.go`) and
    `singleTagQuerier` (`display_test.go`) were updated per Requirement 5 to
    satisfy the new `tagsdb.Querier` signatures (also adding
    `singleTagQuerier.GetTagPurposePolicy`/`UpsertTagPurposePolicy`, which
    task 001 had not touched and which the interface already required), but
    no existing test *assertion* needed to change.
- **Files affected:** `api/service/tag.go`, `api/httpapi/tags.go`,
  `api/service/mock_test.go`, `api/service/display_test.go`,
  `api/service/tag_test.go`, `api/service/tag_one_of_domain_integration_test.go`.
- **Decisions:**
  - Used three small named helpers (`tagFromCreateRow`, `tagFromUpdateColorRow`,
    `tagFromUpdateValueRow`) mirroring the existing `tagFromUUIDRow`
    convention, rather than inline `tagsdb.Tag{...}` literals at each call
    site — keeps `Create`/`UpdateByUUID`/`UpdateTagValue` readable and matches
    the file's existing helper-function idiom, per the task doc's explicit
    "either is fine" guidance.
  - The new integration test reuses `tag_grant_integration_test.go`'s shared
    `TestMain`/`integrationPool`/`integrationSvcs`/`seedActor`/`mustEntityUUID`
    (same package, same build tag) rather than defining its own `TestMain`,
    per the task doc's instruction to reuse the established harness.
  - Seeded the `tag_purpose_policies` row in the integration test via
    `integrationSvcs.TagPurposePolicy.Upsert` (task 001's own service, wired
    into `Services`) rather than a raw SQL insert, since it was already
    available and exercises another piece of Phase 2 in passing.
- **Assumptions applied:** none beyond the task doc's own explicit guidance
  (no `## Assumptions` section was present).

## Metadata

architectural_impact: true

## References

- `plan/overview.md` — Design decisions 3 and 4.
- `api/service/tag.go` — `Tag`, `hydrateTag`, `hydrateTagFromSearchRow`,
  `hydrateTagFromListRow`, `tagFromUUIDRow`, `tagSnapshot`, and all six
  `TagService` methods this task touches.
- `api/httpapi/tags.go` — `tagResponse`, `toTagResponse`.
- `api/service/mock_test.go` — `mockTagQuerier`, the `pgUniqueViolation` constant's
  sibling usage in `tag.go`'s `Create`.
- `api/service/tag_grant_integration_test.go` — the integration-test harness
  conventions (composed migrations, `TestMain`, connectivity probing) to reuse.
- Phase 1 task `002-expose-one-of-domain-in-tag-queries.md` — the query/type changes
  this task consumes.
