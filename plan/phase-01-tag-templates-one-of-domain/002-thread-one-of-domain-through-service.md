# Thread One Of Domain Through Service

## Purpose and scope

Carry the new `ListTagTemplatesRow.OneOfDomain` field through the service layer onto `service.TagTemplate`, teach the test mock to simulate the query's left-join semantics, and add service-layer unit tests for the seeded-`true`, no-policy-row-`false`, and unchanged-behavior cases.

Files touched: `api/service/tag_template.go`, `api/service/mock_test.go`, `api/service/tag_template_test.go`.

**Do not touch `api/service/tag_purpose_policy.go`.** The flag arrives via the SQL join added in task 001; `TagPurposePolicyServicer` is not called by this path and its access-control posture (no `Authorizer` call, no HTTP route) must remain exactly as it is. `TagTemplateService` must likewise keep making no `Authorizer` call — the catalog read stays open.

Covered by the standard implement-task procedure; no special skill.

## Requirements

1. **Add the field to `service.TagTemplate`** in `api/service/tag_template.go`:

   ```go
   OneOfDomain bool
   ```

   Place it after `Scope`. Extend the type's doc comment to state that `OneOfDomain` reports whether the row's `purpose` is one-of-domain per `tag_purpose_policies`, defaulting to `false` when no policy row exists — the same default the DB trigger applies.

2. **Map it in `hydrateTagTemplate`** — a direct `OneOfDomain: r.OneOfDomain` assignment in the struct literal. No other change to `List`.

3. **Leave `Upsert` and `hydrateUpsertedTagTemplate` alone.** `UpsertTagTemplateRow` has no `one_of_domain` column, and the write path is out of scope. An upserted `TagTemplate` therefore carries the zero value `false`; add a one-line comment on `hydrateUpsertedTagTemplate` noting that `OneOfDomain` is deliberately not populated there because the upsert's `RETURNING` clause does not select it.

4. **Update the mock querier** in `api/service/mock_test.go`. `mockTagQuerier` already has a `policies map[string]bool` field and a `seedPurposePolicy(purpose string, oneOfDomain bool)` helper (used today by the `TagPurposePolicyService` tests). Reuse them rather than adding new state: in `ListTagTemplates`, populate each returned row's `OneOfDomain` from `m.policies[t.Purpose]` before appending, so an absent key yields `false` — a faithful simulation of the real `LEFT JOIN` + `COALESCE(..., false)`.

   **Do not change `seedTemplate`'s signature.** Deriving the flag inside `ListTagTemplates` keeps every existing `seedTemplate` call site compiling unchanged, and correctly models the fact that the flag comes from a joined table rather than from the `tag_templates` row.

5. **Add service-layer unit tests** in `api/service/tag_template_test.go`, following the existing table-free `TestTagTemplateService_List_*` naming and style:
   - **Seeded `true`.** `seedPurposePolicy("priority", true)` plus two `priority` templates; assert every returned row has `OneOfDomain == true`. Seed **no tags at all** — this test is the direct proof of the plan's core requirement, that the flag is readable before any real tag of the purpose exists. Say so in the test's doc comment.
   - **Seeded `false`.** `seedPurposePolicy("team", false)` plus a `team` template; assert `OneOfDomain == false`. This distinguishes an explicit `false` row from an absent row at the service boundary.
   - **No policy row.** Templates for a purpose never passed to `seedPurposePolicy`; assert `OneOfDomain == false`.
   - **Existing behavior unchanged.** Confirm the pre-existing `TestTagTemplateService_List_*` tests (globals-only, globals-plus-scoped, unknown-scope, missing-purpose, query-error) still pass untouched. Do not rewrite them; if one needs an assertion added for the new field, add it rather than restructuring the test.

6. **No new exported API beyond the struct field.** No new service method, no new interface method, no signature change to `TagTemplateServicer.List`.

### Toolchain preconditions

From the task worktree root, before building or testing:

```sh
bash scripts/link-siblings.sh
```

`api/go.mod` reaches `mod-core` through sibling-relative replace directives that do not resolve in a fresh worktree until these symlinks exist.

## Validation

- `cd api && go build ./...` compiles.
- `cd api && go test ./...` passes, including the new tests and every pre-existing `TestTagTemplateService_List_*` test.
- `cd api && go vet ./...` is clean and `gofmt -l .` reports nothing.
- `grep -n "OneOfDomain" api/service/tag_template.go` shows exactly the struct field, the doc-comment mention, and the `hydrateTagTemplate` assignment.
- `git diff --stat api/service/tag_purpose_policy.go` is empty — that file must be untouched.
- `grep -n "Authorize" api/service/tag_template.go` returns nothing — the catalog read remains open, unchanged.
- The new tests fail if the `hydrateTagTemplate` assignment is reverted (sanity-check by temporarily removing it, then restore).

## Metadata

architectural_impact: false

## Assumptions

- Task 001 has landed, so `tagsdb.ListTagTemplatesRow` already carries `OneOfDomain bool`. Without it this task does not compile — if the field is absent, halt and report rather than editing `model/db/` by hand.

## References

- `api/service/tag_template.go` — `TagTemplate`, `TagTemplateServicer`, `List`, `hydrateTagTemplate`, `hydrateUpsertedTagTemplate`.
- `api/service/mock_test.go` — `mockTagQuerier`'s `policies` map (~line 461), `seedPurposePolicy`, `seedTemplate` (~line 730), and `ListTagTemplates` (~line 755).
- `api/service/tag_template_test.go` — the existing `TestTagTemplateService_List_*` tests whose style the new ones follow.
- `api/service/tag_purpose_policy.go` — the file this task must **not** modify; its doc comments state the no-route, no-`Authorizer` posture being preserved.
- `docs/decisions/tags-one-of-domain.md` — the `one_of_domain` default and the no-public-write-endpoint constraint.
