# Add Catalog One Of Domain Integration Test

## Purpose and scope

Prove against a live Postgres that the rewritten `ListTagTemplates` query actually returns the `tag_purpose_policies` flag — that the `LEFT JOIN` resolves, that a seeded `one_of_domain = true` purpose reads back `true`, and that a purpose with no policy row reads back `false`. Unit tests exercise the mock's simulation of the join; only this test exercises the real SQL.

This is the plan's **model-layer** coverage. `model/` has no Go test suite of its own (`make -C model test` prints "no unit tests (generated code)"), so DB-level verification for this module lives in `api/service/*_integration_test.go` behind the `integration` build tag.

New file: `api/service/tag_template_one_of_domain_integration_test.go`. No production code changes.

Covered by the standard implement-task procedure; no special skill.

## Requirements

1. **Create `api/service/tag_template_one_of_domain_integration_test.go`** with the `//go:build integration` constraint and `package service`, matching the two existing integration files.

2. **Reuse the shared harness; do not define a `TestMain`.** `api/service/tag_grant_integration_test.go` owns the package's single `TestMain`, which builds the ephemeral composed-migrations directory, probes for a reachable Postgres host, and initializes the package-level `integrationPool` and `integrationSvcs`. Follow `tag_one_of_domain_integration_test.go`'s doc-comment convention of stating that it reuses that harness and pointing at it.

3. **Write one test, `TestTagTemplateCatalogOneOfDomainIntegration`**, covering all three required cases in one live-DB session:

   - **Seeded `true`.** Seed a `tag_purpose_policies` row via `integrationSvcs.TagPurposePolicy.Upsert(ctx, tagQ, purpose, true)` for a test-unique purpose, seed at least one `tag_templates` row for that purpose, then call `integrationSvcs.TagTemplate.List(...)` and assert every returned row has `OneOfDomain == true`.

     **Create no `tags` rows in this test.** That is the point of the whole change: the flag must be readable for a purpose that has catalog entries but no real tag yet. State this in the test's doc comment so a later reader does not "helpfully" add tag seeding.

   - **Explicit `false`.** A second purpose upserted with `false`; assert `OneOfDomain == false`.

   - **No policy row.** A third purpose never passed to `Upsert`; assert `OneOfDomain == false`, confirming the `COALESCE` default matches the DB trigger's own `NOT FOUND → false` fallback.

   Use distinct, obviously-test-scoped purpose strings (the existing file uses `one-of-domain-verify`) so the test does not collide with rows any sibling integration test seeds in the shared database.

4. **Seed templates through existing helpers.** `seedApp(ctx, pool, slug, name)` already exists in `api/service/tag_template_upsert_integration_test.go` (same package, same build tag) and inserts a real `entities` + `apps` pair satisfying `tag_templates.scope`'s FK. Pair it with `integrationSvcs.TagTemplate.Upsert` for scoped rows. If a global (NULL-scope) template is wanted as well, insert it with a direct `pool.Exec` — `TagTemplate.Upsert` deliberately rejects a nil scope.

5. **Cover both the scoped and the global path if cheap**, since the query's join sits alongside the pre-existing `LEFT JOIN entities e` and a regression there would show up as a wrong `scope`/`scope_uuid`. Assert the existing `Scope` field still hydrates correctly on at least one row — proof the added join did not disturb the existing one.

6. **No production code changes.** If this test fails, the fix belongs in task 001's query, not here — halt and report rather than adjusting the query from this task.

### Toolchain preconditions

From the task worktree root:

```sh
bash scripts/link-siblings.sh
```

Run the suite from `mod-tags/api`:

```sh
go test -tags=integration -p 1 -v ./service/... -run TestTagTemplateCatalogOneOfDomainIntegration
```

`-p 1` is mandatory — `TestMain` drops and recreates its verification database, so concurrent packages against the same host conflict. The suite needs a reachable Postgres; `TAGS_DEV_PG_HOST` overrides the host probe. If no database is reachable in this environment, the test must still **compile** (`go vet -tags=integration ./service/...`) — report the un-run state rather than deleting or weakening the test.

## Validation

- `cd api && go vet -tags=integration ./service/...` is clean — this compiles the new file even where no database is available, and is the minimum bar for the task.
- `cd api && gofmt -l .` reports nothing.
- `cd api && go test ./...` (no build tag) is unaffected — the new file is excluded by its build constraint.
- Where a live Postgres is reachable: `cd api && go test -tags=integration -p 1 -v ./service/... -run TestTagTemplateCatalogOneOfDomainIntegration` passes, and the pre-existing `TestTagOneOfDomainIntegration` and `TestTagTemplateUpsertIntegration` still pass in the same run.
- `grep -n "func TestMain" api/service/*_integration_test.go` still shows exactly one definition (in `tag_grant_integration_test.go`).
- `grep -n "CreateTag\|TagService.Create\|INSERT INTO tags" api/service/tag_template_one_of_domain_integration_test.go` returns nothing — the test must not create any real tag.

## Assumptions

- Tasks 001 and 002 have landed. Without task 002's `service.TagTemplate.OneOfDomain` field there is nothing to assert against.
- A live Postgres may not be reachable in the executing environment. That is an expected, reportable outcome, not a reason to change the test's design.

## References

- `api/service/tag_grant_integration_test.go` — the shared `TestMain`, `integrationPool`, `integrationSvcs`, `seedActor`, host probing, and ephemeral composed-migrations directory.
- `api/service/tag_one_of_domain_integration_test.go` — the closest existing model: seeds a `tag_purpose_policies` row via `integrationSvcs.TagPurposePolicy.Upsert` and asserts live-DB behavior.
- `api/service/tag_template_upsert_integration_test.go` — the `seedApp` helper and the `TagTemplate.Upsert` invocation pattern.
- `model/migrations/0205_tags_one_of_domain.sql` — the trigger whose `NOT FOUND → false` fallback the no-policy-row case must match.
