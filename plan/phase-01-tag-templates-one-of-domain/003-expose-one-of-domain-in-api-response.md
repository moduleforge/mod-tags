# Expose One Of Domain In Api Response

## Purpose and scope

Emit the `oneOfDomain` flag on every `GET /tag-templates` response row, update the OpenAPI fragment's `TagTemplate` schema to match, and add handler tests asserting the JSON key. This is the task that actually delivers the field app-mftodo consumes.

Files touched: `api/httpapi/tag_templates.go`, `api/openapi.fragment.yaml`, `api/httpapi/tag_templates_test.go`.

Covered by the standard implement-task procedure; no special skill.

## Requirements

1. **Add the field to `tagTemplateResponse`** in `api/httpapi/tag_templates.go`:

   ```go
   OneOfDomain bool `json:"oneOfDomain"`
   ```

   The JSON key is `oneOfDomain` — camelCase, matching `api/httpapi/tags.go`'s existing `tagResponse.OneOfDomain` key exactly. Place it after `Scope`. Map it in `toTagTemplateResponse` with a direct assignment.

2. **Keep the response a bare JSON array.** Do **not** introduce an envelope (`{templates: [...], oneOfDomain: ...}`), a `purposePolicies` map, or any other restructuring. The field is additive per-row so existing consumers that ignore the key are unaffected. `next-steps.md` records a separate, deliberately-deferred "list envelope asymmetry" standardization — that is not this change, and attempting it here would turn an additive fix into a breaking API change.

3. **Change nothing else in the handler.** No new query parameter, no new error path, no change to the `401`/`400` guards, no change to status codes or ordering. `handleListTagTemplates` keeps requiring an authenticated actor and keeps making no `Authorizer` call.

4. **Update `api/openapi.fragment.yaml`.** In `components.schemas.TagTemplate`:
   - Add a `oneOfDomain` property: `type: boolean`, with a description stating it reports whether the row's `purpose` is one-of-domain (an owner may hold at most one tag of that purpose on a subject), sourced from the admin-curated `tag_purpose_policies` registry and `false` when no policy row exists.
   - Add `oneOfDomain` to the schema's `required` list (it is a non-nullable bool always present on the wire), alongside the existing `purpose, value, label, sortOrder`.
   - Extend the `/tag-templates` `get.description` with one sentence noting the flag is read-only here and that the registry has no public write endpoint.

5. **Add handler tests** in `api/httpapi/tag_templates_test.go`, following the existing `TestHandleListTagTemplates_*` naming and the `map[string]any` body-decoding style already used there:
   - A response built from `fakeTagTemplateService` templates with `OneOfDomain: true` decodes to `row["oneOfDomain"] == true`.
   - A response built from templates with `OneOfDomain: false` decodes to `row["oneOfDomain"] == false` and the key is **present**, not omitted — assert presence explicitly with the two-value map lookup, since a wrongly-added `omitempty` would silently drop a `false` and is exactly the bug this assertion must catch.
   - The existing tests (`400_MissingPurpose`, `200_PurposeOnly_Globals`, `200_PurposeAndScope_GlobalsPlusScoped`, `400_MalformedScope`, `200_EmptyResult`, `500_ServiceError`, `401_Unauthenticated`) still pass untouched.

6. **`fakeTagTemplateService` needs no change** — it returns caller-supplied `service.TagTemplate` values, which already carry `OneOfDomain` after task 002. Set the field in each test's literal.

### Toolchain preconditions

From the task worktree root, before building or testing:

```sh
bash scripts/link-siblings.sh
```

## Validation

- `cd api && go build ./...` compiles.
- `cd api && go test ./...` passes, including every pre-existing `TestHandleListTagTemplates_*` test.
- `cd api && go vet ./...` is clean and `gofmt -l .` reports nothing.
- `grep -n "oneOfDomain" api/httpapi/tag_templates.go` shows the struct tag; `grep -rn "omitempty" api/httpapi/tag_templates.go` returns nothing on the new field.
- The JSON tag spelling matches the existing tag response: `grep -n "oneOfDomain" api/httpapi/tags.go api/httpapi/tag_templates.go` shows the identical key in both.
- `api/openapi.fragment.yaml` parses as valid YAML and its `TagTemplate` schema lists `oneOfDomain` in both `properties` and `required`.
- Manual shape check: the 200 body is still a JSON array (starts with `[`), not an object — assert this in one of the new tests by decoding into `[]map[string]any` as the existing tests already do.

## Metadata

architectural_impact: true

## Assumptions

- Tasks 001 and 002 have landed, so `service.TagTemplate` carries `OneOfDomain bool`.

## Status

- **Outcome:** succeeded
- **Date:** 2026-08-07
- **Validation summary:** `go build ./...`, `go test ./...`, and `go vet ./...` all pass under `api/`; `gofmt -l .` reports nothing. All pre-existing `TestHandleListTagTemplates_*` tests pass unmodified alongside two new tests (`_200_OneOfDomainTrue`, `_200_OneOfDomainFalse`). `oneOfDomain` grep checks confirm the key spelling matches `tags.go` exactly and carries no `omitempty`. `api/openapi.fragment.yaml` parses as valid YAML with `oneOfDomain` present in both `properties` and `required` on `TagTemplate`.
- **Affected files:**
  - `api/httpapi/tag_templates.go`
  - `api/openapi.fragment.yaml`
  - `api/httpapi/tag_templates_test.go`
- **Assumptions applied:** Tasks 001 and 002 had already landed on the plan branch this worktree was cut from, so `service.TagTemplate.OneOfDomain` was present with no further work needed to make it available to the handler.

## References

- `api/httpapi/tag_templates.go` — `tagTemplateResponse`, `toTagTemplateResponse`, `handleListTagTemplates`.
- `api/httpapi/tags.go` — the pre-existing `OneOfDomain bool \`json:"oneOfDomain"\`` on `tagResponse`; the key spelling to match.
- `api/httpapi/tag_templates_test.go` — the existing handler tests and their decoding style.
- `api/openapi.fragment.yaml` — the `/tag-templates` path and the `TagTemplate` schema.
- `plan/notes/tag-templates-response-shape.md` — why per-row rather than an envelope.

## Checkpoint hints

- After the `tagTemplateResponse` / `toTagTemplateResponse` edit.
- After the `api/openapi.fragment.yaml` schema and description updates.
- After the new handler tests pass.
