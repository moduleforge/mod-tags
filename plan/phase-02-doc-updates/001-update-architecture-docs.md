# Update Architecture Docs

## Purpose and scope

Reconcile mod-tags' architecture-of-record with the API surface Phase 1 lands: `GET /tag-templates` now exposes the per-purpose `one_of_domain` flag, which the standing decision record says reaches clients only through tag-returning responses. Left unreconciled, the decision record actively misleads the next reader about where the flag is visible and about whether any route reads `tag_purpose_policies`.

Follow the `update-architecture-docs` task-procedure at `plugins/flow/task-procedures/update-architecture-docs/SKILL.md`.

## Requirements

**role_doc:** `plugins/flow/roles/architect-backend.md`

The implications are API-surface and component-boundary changes on an existing HTTP route — no new subsystem, no schema change, no infrastructure or topology change — so the backend architect role applies rather than the data, cloud, or frontend variants.

### Which implementation tasks surfaced these implications

All paths relative to the plan worktree:

- `plan/phase-01-tag-templates-one-of-domain/001-add-one-of-domain-to-list-query.md` — the `ListTagTemplates` query now joins `tag_purpose_policies`, making the catalog read a reader of the policy registry for the first time.
- `plan/phase-01-tag-templates-one-of-domain/003-expose-one-of-domain-in-api-response.md` — the public HTTP response for `GET /tag-templates` gains a field, and `api/openapi.fragment.yaml` changes with it.

Both will have landed by the time this phase runs; read the merged code, not only these task docs.

### Which docs to review and update

1. **`docs/decisions/tags-one-of-domain.md`** — the primary target. Two passages are now inaccurate or incomplete:
   - The **"API surface"** section's *"Exposing 'occupied' purposes to the GUI"* bullet argues a per-tag field "was chosen over a new lookup endpoint" because "'occupied' is only meaningful for a purpose that already has an existing tag on the subject." That premise is precisely what the regression disproved: a client needs to know a purpose is one-of-domain *before* any tag of it exists, in order to guard a picker. Record the addendum without rewriting the original reasoning as if it had never been held — the per-tag field remains correct for the "occupied" question; what is added is a *catalog-time* answer to the different "is this purpose exclusive at all" question, on an endpoint the client already calls to populate the picker. Note explicitly that no new endpoint was added.
   - The **"No public write endpoint for the policy flag itself"** bullet says `Upsert`/`Get` "are internal/administrative-only capabilities — no HTTP route calls them", framed as covering the whole table. Tighten it to the write path, which is what the constraint actually protects: still no route writes `tag_purpose_policies`, still no `Upsert` route, `TagPurposePolicyServicer`'s access-control posture is unchanged, and — worth stating, since it is the subtle part — the flag now reaching clients does so via a SQL join in `ListTagTemplates`, not by any route calling `TagPurposePolicyServicer.Get`, so that servicer genuinely still has no caller behind an HTTP route.
   - Extend the **"Consequences"** table with a row for the catalog read's new visibility of the flag.

2. **`docs/project-roadmap.md`** — review only. Its `one_of_domain` mention sits inside the deferred `tag_qualifier_policies` / scoped-variant discussion. Confirm nothing there now contradicts the landed change (in particular that no roadmap item claims catalog exposure as future work); update only if it does. A no-change outcome is a valid, reportable result.

3. **Confirm the absence of a project-wide architecture or spec doc rather than assuming it.** Run `ls docs/architecture.md docs/*-spec.md 2>/dev/null` from the module root. mod-tags is expected to have **neither** — `docs/architecture.md` and `docs/architecture/` belong to `docs/mf-standards/`, a **git submodule pointing at a separate repository**. If the glob turns up a mod-tags-owned file, review and update it too; otherwise record the absence in the report and do not create one.

4. **Never edit anything under `docs/mf-standards/`.** It is a submodule. If the change genuinely warrants an update there (it should not — this is a module-local API addition, not a platform-standard change), flag it for the manager instead of editing.

5. **Do not duplicate Phase 1's doc work.** `README.md`, `AGENTS.md`, `next-steps.md`, and `api/openapi.fragment.yaml` were updated in Phase 1. Read them to stay consistent; change them here only to fix an inconsistency Phase 1 left behind, and say so in the report if you do.

## Validation

- `docs/decisions/tags-one-of-domain.md` no longer states or implies that the `one_of_domain` flag is visible to clients solely through tag-returning responses.
- `grep -n "no HTTP route calls them\|no HTTP route" docs/decisions/tags-one-of-domain.md` — every surviving instance is scoped to the write path, and none of them contradicts the landed read path.
- The decision record's account of `TagPurposePolicyServicer` matches the code: `grep -rn "TagPurposePolicy" api/httpapi/` returns nothing, confirming no handler references it.
- `ls docs/architecture.md docs/*-spec.md` result is recorded in the report, whichever way it comes out.
- `git diff --stat` lists only files under `docs/` owned by mod-tags. `git status --short docs/mf-standards` shows the submodule unmodified.
- Every relative link introduced or touched resolves from the file's own directory.
- The decision record's existing structure (`## Purpose and scope`, `## Context`, `## Decision`, `## Consequences`) is preserved, and the addendum reads as a decision *evolution* with its date and driver, not as a silent retcon of the original text.

## References

- `docs/decisions/tags-one-of-domain.md` — the primary document under review; its "API surface" and "Consequences" sections.
- `docs/project-roadmap.md` — the deferred scoped-variant and `tag_qualifier_policies` discussion.
- `plan/overview.md` — the plan's design decisions D1–D3 and the hard constraints that must still hold after the edit.
- `plan/notes/tag-templates-response-shape.md` — the evidence behind the per-row shape and the SQL-sourcing choice.
- `api/httpapi/tag_templates.go`, `api/service/tag_template.go`, `model/queries/tag_templates.sql` — the landed implementation the docs must now describe.
