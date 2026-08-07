# Tag-Templates Response Shape

## Purpose and scope

Records the codebase survey behind the two load-bearing design choices in [the plan overview](../overview.md): how the `one_of_domain` flag is carried on the `GET /tag-templates` response (D1), and where it is sourced from (D2). Written during planning so implementing agents do not re-derive it.

## Current endpoint shape

`api/httpapi/tag_templates.go` — `handleListTagTemplates`:

- Rejects a request with no actor in context with `401 unauthenticated` (`opctx.ActorEntityID`), then requires a non-empty `purpose` query parameter (`400` otherwise), then optionally parses `scope` as a UUID (`400` on malformed).
- Calls `service.TagTemplateServicer.List` and writes a **bare JSON array** of `tagTemplateResponse` at `200`.

`tagTemplateResponse` today: `purpose`, `value`, `label`, `color`, `sortOrder`, `scope`.

**Note on "unauthenticated".** The plan request describes this route as "open, unauthenticated". It is open in the sense that it applies **no per-row authorization** — that is what `README.md`, `AGENTS.md`, and the `TagTemplateService` doc comments mean by "open" — but it does require an authenticated actor, and `api/httpapi/tag_templates_test.go`'s `TestHandleListTagTemplates_401_Unauthenticated` locks that in deliberately. This change does not alter that posture either way, so the discrepancy is descriptive only.

## D1 — Alternatives for carrying the flag

| Option | Verdict |
|---|---|
| Per-row `oneOfDomain` on each array element | **Chosen.** Additive; matches the module's own tag wire shape. |
| Envelope `{templates: [...], oneOfDomain}` | Rejected — breaking change to a shipped endpoint. |
| Sibling `purposePolicies` map | Rejected — same breaking-change problem, and pointless when `purpose` is a required single-valued parameter. |

The decisive precedent is `api/httpapi/tags.go`, whose `tagResponse` already carries:

```go
OneOfDomain bool `json:"oneOfDomain"`
```

`docs/decisions/tags-one-of-domain.md` records that this per-row placement was applied uniformly across *all* tag-returning endpoints specifically to keep the wire shape consistent module-wide. Extending the same key to the catalog row is the consistent move; introducing a second, differently-shaped way to express the same flag on the same module's API is not.

`next-steps.md` separately notes a "list envelope asymmetry" (`GET /tags` returns a bare array, `GET /entities/{uuid}/tags` returns `{tags: [...]}`) as something "worth standardizing in a future pass". That standardization is explicitly a separate future concern and must not be attempted here — wrapping `/tag-templates` now would make this bug-fix a breaking API change.

## D2 — Alternatives for sourcing the flag

| Option | Verdict |
|---|---|
| `LEFT JOIN tag_purpose_policies` in `ListTagTemplates` | **Chosen.** One query; touches no service. |
| Service-layer `TagPurposePolicyServicer.Get` call from `TagTemplateService.List` | Rejected — see below. |

`model/queries/tags.sql` already establishes both spellings of the SQL pattern:

- `:many` queries (`ListTagsBySubjectEntityID`, `SearchTags`) and the `:one` `GetTagByEntityUUID` use `LEFT JOIN tag_purpose_policies tpp ON tpp.purpose = t.purpose` with `COALESCE(tpp.one_of_domain, false) AS one_of_domain`.
- `RETURNING`-clause queries (`CreateTag`, `UpdateTagColor`, `UpdateTagValue`), which cannot carry a join, use a correlated subquery with an explicit `::boolean` cast.

`ListTagTemplates` is a `:many` with an existing `LEFT JOIN entities e`, so the join spelling is the exact match. sqlc types the join form's `COALESCE(...)` as a plain Go `bool` without needing the cast — confirmed by `model/db/tags.sql.go`, where `ListTagsBySubjectEntityIDRow.OneOfDomain` is `bool`.

The service-layer alternative was rejected on three counts: it would add a second query per request; it would require `TagTemplateService.List` to depend on `TagPurposePolicyServicer`, changing that type's role from "no HTTP-exposed route calls it" to "reached on every catalog read"; and the plan's hard constraint is to leave `TagPurposePolicyServicer`'s posture untouched. The SQL approach satisfies that constraint by construction — `api/service/tag_purpose_policy.go` is not edited at all.

## Where the default comes from

`COALESCE(..., false)` reproduces exactly the fallback the enforcement trigger applies. `model/migrations/0205_tags_one_of_domain.sql`'s `tags_enforce_one_of_domain()` sets `v_one_of_domain := false` on `NOT FOUND`, and `TagPurposePolicyService.Get` independently returns `OneOfDomain: false` on `pgx.ErrNoRows`. All three agree, which is what makes "no policy row reads as `false`" a testable invariant rather than an incidental behavior.

## What already exists and needs no work

- `tag_purpose_policies` table, its `updated_at` trigger, and the `BEFORE INSERT` enforcement trigger — `model/migrations/0205_tags_one_of_domain.sql`.
- `GetTagPurposePolicy` / `UpsertTagPurposePolicy` queries and their generated code — `model/queries/tag_purpose_policies.sql`, `model/db/tag_purpose_policies.sql.go`.
- `TagPurposePolicyServicer` and its wiring into `service.Services` — `api/service/tag_purpose_policy.go`, `api/service/service.go`.
- Seeded policy rows — consuming apps seed out-of-band; app-mftodo already seeds `('priority', true)`.

No migration is required by this change.
