# Update Module Endpoint Docs

## Purpose and scope

Bring the module's consumer-facing docs in line with the `oneOfDomain` field now on the `GET /tag-templates` response, so an integrating app can discover it without reading the handler.

Files touched: `README.md` and `AGENTS.md` at the module root, and `next-steps.md` where it currently overstates the policy registry's invisibility.

Documentation only — no code, no tests, no behavior change.

Covered by the standard implement-task procedure; no special skill.

## Requirements

1. **`AGENTS.md` — the endpoint list.** The "Router mounting" section's bullet for `GET /tag-templates` currently reads:

   > `GET /tag-templates` — list tag-template catalog entries by purpose (optionally scoped to an app); an open, catalog-only read with no per-row authorization, unlike the tag routes above

   Extend it to say each entry also carries the purpose's read-only `oneOfDomain` flag, sourced from the admin-curated `tag_purpose_policies` registry and `false` when no policy row exists. Keep the existing wording about per-row authorization intact.

2. **`AGENTS.md` — the Conventions section.** The bullet stating authorization is checked first "the one exception is the open, catalog-only `GET /tag-templates` read, which intentionally makes no `Authorizer` call" is still accurate and must **not** be weakened or removed. If anything is added there, it is only a clarification that reading the `one_of_domain` flag through this route introduces no authorization change and no write path.

3. **`README.md` — Core features.** The "Tag-templates catalog" bullet gains a mention of the `oneOfDomain` flag on each entry. The adjacent "Per-purpose exclusivity (`one_of_domain`)" bullet currently says the flag is "enforced at the DB (trigger), API, and GUI layers" — extend it to note the flag is now also *readable* per purpose from the tag-templates catalog, which is what lets a client know a purpose is exclusive before any tag of it exists.

4. **`next-steps.md` — precision fix.** The "Known carry-forward items" bullet titled **`tag_purpose_policies` has no public write endpoint** says rows "must be seeded out-of-band ... until/unless a curated admin surface is designed". That remains true and must stay. Adjust only the surrounding wording so it is clear the *write* path is what has no public endpoint — the flag is now publicly *readable* via `GET /tag-templates`. Do not delete the bullet, and do not add a new item claiming the work is done.

5. **Consistency of terminology.** Use `oneOfDomain` when naming the JSON field on the wire and `one_of_domain` when naming the database column or the policy concept, matching how the existing docs already distinguish the two.

6. **Do not touch these:**
   - `docs/decisions/tags-one-of-domain.md` and `docs/project-roadmap.md` — owned by the `doc-updates` phase.
   - `docs/mf-standards/` — a git submodule pointing at a separate repository.
   - `api/openapi.fragment.yaml` — updated by task 003.
   - `api/README.md` and `model/README.md` — neither documents the endpoint list or the response shape, so neither needs a change; confirm with a grep rather than assuming.

## Validation

- `grep -n "oneOfDomain" README.md AGENTS.md` shows the new mentions.
- `grep -n "tag-templates" AGENTS.md README.md` shows the endpoint still described as an open, catalog-only read with no per-row authorization — that characterization must survive the edit.
- `grep -rn "one_of_domain\|oneOfDomain" api/README.md model/README.md` confirms whether those two files mention the response shape at all; if they do not, leave them unchanged and note it.
- `git diff --stat` lists only `README.md`, `AGENTS.md`, and `next-steps.md`.
- No claim in the diff asserts a write endpoint, an admin surface, or a seeding capability now exists in mod-tags — none does.
- Markdown renders cleanly: relative links resolve, code spans are balanced, no stray table-column mismatches.

## Assumptions

- Task 003 has landed, so the field described actually exists on the wire.

## References

- `AGENTS.md` — "Router mounting" endpoint list and the "Conventions" authorization bullet.
- `README.md` — "Core features" and "Integration guide".
- `next-steps.md` — the "`tag_purpose_policies` has no public write endpoint" carry-forward item.
- `api/httpapi/tag_templates.go` — the authoritative response shape after task 003.
