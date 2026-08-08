# Tags One-Of-Domain Decision Record

## Purpose and scope

This decision record documents `one_of_domain`: a per-`purpose` boolean policy that
governs whether an owner may attach more than one tag of a given `purpose` to the
same subject at a time. The originating request used the word "domain" for this
concept; that word was clarified directly with the requester to mean the existing
`purpose` column on the `tags` table
([`model/migrations/0201_tags.sql`](../../model/migrations/0201_tags.sql)) — this
feature does not introduce a separate "domain" table or column. This record covers
the `tag_purpose_policies` schema shape, the default applied when no policy row
exists, the trigger-based enforcement mechanism, and the API and GUI surface that
expose and consume the policy.

## Context

Before this change, `tags` carried an unconditional
[`tags_owner_subject_purpose_idx`](../../model/migrations/0201_tags.sql) unique index
on `(owner_id, subject_id, purpose)`: for every purpose, an owner could never attach
two tags of the same purpose to the same subject. That blanket rule needed to become
*conditional* — some purposes (e.g. "priority") should stay exclusive, while others
(e.g. "team") should allow multiple concurrent tags on the same subject — gated per
purpose by a new, admin-curated flag rather than hard-coded in the schema.

## Decision

### The `tag_purpose_policies` table

`one_of_domain` lives in a new table, `tag_purpose_policies (purpose TEXT PRIMARY
KEY, one_of_domain BOOLEAN NOT NULL DEFAULT false, created_at, updated_at)`, keyed
purely on `purpose` — no `scope` column.

It is deliberately not a column on `tag_templates`. `tag_templates` is keyed per
`(scope, purpose, value)` — a finer grain than `purpose` alone — and is an
explicitly open, read-only, non-authorization-gated suggestion catalog for UI
pickers (see [`model/README.md`](../../model/README.md) and this project's
[README](../../README.md#core-features)). Putting an enforcement-relevant flag there
would require either forcing every row sharing a `purpose` to agree on the flag (new
constraint machinery, with an unclear answer for whether a `scope`-scoped row could
disagree with a global row for the same purpose), or a value-less placeholder row,
which doesn't fit `tag_templates`' `value NOT NULL` plus `(scope, purpose, value)`
unique design. A separate table keyed only on `purpose` avoids both problems and
keeps the read-only suggestion catalog semantically pure.

`tag_purpose_policies` also has no `scope` column: purpose semantics (e.g.
"priority", "status") are conceptually global, not per-app. This is a deliberate
scoping decision, not an oversight — see
[`docs/project-roadmap.md`](../project-roadmap.md) for how a future scoped variant
would be a separate, not-yet-designed extension.

**Naming note.** [`docs/project-roadmap.md`](../project-roadmap.md) already
anticipates an unrelated, deferred `tag_qualifier_policies` table (open/closed
*value*-catalog enforcement — whether arbitrary values are accepted for a purpose at
all). `tag_purpose_policies` is conceptually distinct: it governs *how many* tags of
a purpose may coexist, not *which values* are allowed. The roadmap document calls
out this distinction explicitly so the two are not conflated later.

### Default for unregistered purposes

**`one_of_domain = false`** (multiple tags of that purpose allowed) is the default
applied when no `tag_purpose_policies` row exists for a purpose. This is not a
column default alone — the column does default to `false` — but is also the
fallback the enforcement trigger applies on a `NOT FOUND` lookup (see below), so the
same default holds whether or not a row is ever inserted for a given purpose.

### Enforcement mechanism

A plain (partial) unique index cannot express "conditionally unique based on a value
looked up in another table." Enforcement instead lives in a `BEFORE INSERT` trigger
on `tags`, mirroring this module's existing `tags_check_type()` /
`tags_reject_immutable_changes()` trigger style
([`model/migrations/0201_tags.sql`](../../model/migrations/0201_tags.sql)):

- The previous unconditional `UNIQUE INDEX tags_owner_subject_purpose_idx` is
  dropped and replaced by a **plain (non-unique) index of the same name and
  columns** — the index itself remains, only its uniqueness is removed, so lookup
  performance for the trigger's own check (and any other `(owner_id, subject_id,
  purpose)` query) is unaffected.
- A new trigger function, `tags_enforce_one_of_domain()`, looks up `one_of_domain`
  for `NEW.purpose` (defaulting to `false` when absent) and, only when `true`,
  rejects the insert if a tag already exists for `(owner_id, subject_id, purpose)`.
- **`BEFORE INSERT` only — not `UPDATE`.** `purpose` (and `owner_id`/`subject_id`)
  are already immutable after insert via `tags_reject_immutable_changes` (see
  [`tags-limited-immutability.md`](./tags-limited-immutability.md)), so no `UPDATE`
  path can create a new one-of-domain conflict; only `INSERT` needs the check.
- **Race-safety.** A trigger-based `EXISTS` check has a TOCTOU gap a true unique
  index does not: two concurrent inserts for the same `(owner_id, subject_id,
  purpose)` could each pass the check before either commits. The trigger closes
  this by taking a transaction-scoped advisory lock (`pg_advisory_xact_lock`, keyed
  on a hash of the tuple) before the `EXISTS` check, serializing concurrent inserts
  for the same tuple. This is a deliberate, documented mitigation — a known
  characteristic of the trigger-based approach relative to a true unique index,
  accepted because a true conditional unique index is not expressible in Postgres.
- **SQLSTATE reuse.** The trigger raises its exception `USING ERRCODE =
  'unique_violation'` (the standard `23505` code) — the *same* SQLSTATE the
  previous unconditional unique index violation raised, deliberately, so
  `api/service/tag.go`'s existing `pgErr.Code == pgUniqueViolation` →
  `ErrConflict` classification in `TagService.Create` requires **no Go code
  changes** to correctly map a one-of-domain conflict to `409 conflict`.

The full trigger, as landed in
[`model/migrations/0205_tags_one_of_domain.sql`](../../model/migrations/0205_tags_one_of_domain.sql):

```sql
CREATE FUNCTION tags_enforce_one_of_domain() RETURNS TRIGGER AS $$
DECLARE
  v_one_of_domain BOOLEAN;
BEGIN
  SELECT one_of_domain INTO v_one_of_domain
  FROM tag_purpose_policies
  WHERE purpose = NEW.purpose;

  IF NOT FOUND THEN
    v_one_of_domain := false;
  END IF;

  IF v_one_of_domain THEN
    PERFORM pg_advisory_xact_lock(
      hashtextextended(NEW.owner_id::text || ':' || NEW.subject_id::text || ':' || NEW.purpose, 0)
    );

    IF EXISTS (
      SELECT 1 FROM tags
      WHERE owner_id = NEW.owner_id
        AND subject_id = NEW.subject_id
        AND purpose = NEW.purpose
    ) THEN
      RAISE EXCEPTION 'tags: purpose % is one-of-domain; owner % already has a tag of this purpose on subject %',
        NEW.purpose, NEW.owner_id, NEW.subject_id
        USING ERRCODE = 'unique_violation';
    END IF;
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tags_enforce_one_of_domain
  BEFORE INSERT ON tags
  FOR EACH ROW EXECUTE FUNCTION tags_enforce_one_of_domain();
```

The BEFORE INSERT trigger that validates entity type is named `tags_type_check`
(`tags_check_type` is its trigger *function*, not the trigger name Postgres uses
for same-event firing order). Comparing the actual trigger names,
`tags_enforce_one_of_domain` sorts alphabetically *before* `tags_type_check`
(`e` < `t` after the shared `tags_` prefix), so on insert the one-of-domain check
actually runs first and entity-type validation runs second — the reverse of what
might be expected, though harmless here since the one-of-domain check only reads
`NEW.owner_id`/`NEW.subject_id`/`NEW.purpose`, none of which depend on the
type-check trigger's effects. `tags_reject_immutable_changes` and
`tags_set_updated_at` fire on `BEFORE UPDATE`, a different event, so they never
compete with this trigger's `BEFORE INSERT` firing order at all.

### API surface

- **Conflict surfacing.** No Go code changes were needed for the error
  *classification* itself, per the SQLSTATE reuse above; `api/service/tag.go`'s
  existing `pgErr.Code == pgUniqueViolation` check already maps a one-of-domain
  conflict to `ErrConflict` (409).
- **Exposing "occupied" purposes to the GUI.** Every tag-returning response (the
  `Tag` service type, and therefore `POST /tags`, `GET /tags/{uuid}`, `GET /tags`,
  `GET /entities/{uuid}/tags`, `PUT /tags/{uuid}`, `PATCH /tags/{uuid}`) carries a
  new `oneOfDomain: boolean` field per tag, computed via a join/subquery against
  `tag_purpose_policies` in the underlying `sqlc` queries. This was chosen over a
  new lookup endpoint: "occupied" is only meaningful for a purpose that already has
  an existing tag on the subject — exactly what `GET /entities/{uuid}/tags` (which
  the GUI already calls on every `TagEditor`/`TagList` mount) enumerates. Embedding
  the flag avoids a second round trip and a new endpoint, and applying it uniformly
  across all tag-returning endpoints (not just the list one) keeps the wire `Tag`
  shape consistent module-wide.
- **Addendum (2026-08-07): catalog-time exposure on `GET /tag-templates`.** The
  "occupied" reasoning above answers a different question than the one a picker
  needs answered *before* any tag exists. A client populating a purpose picker
  (mod-tags' own `TagEditor`, or a consuming app's own combobox) must know whether
  a purpose is one-of-domain at all — to decide, for example, whether to disable
  re-selection once one instance of that purpose is chosen — and no tag of that
  purpose need exist yet for that question to matter. The per-tag `oneOfDomain`
  field above cannot answer it: a purpose with zero tags on the subject never
  appears in a tag-returning response, so a client has no `oneOfDomain` value to
  read until after the exclusivity has already been violated once. Driven by a
  regression this gap caused in a consuming app's task-creation tag picker, `GET
  /tag-templates` — the catalog endpoint pickers already call to populate their
  options — now also carries a per-row `oneOfDomain: boolean`, sourced via a `LEFT
  JOIN tag_purpose_policies` in the `ListTagTemplates` query (`model/queries/
  tag_templates.sql`), the same `COALESCE(..., false)` default as the per-tag
  field. This does not revise the "occupied" reasoning above, which remains correct
  for its own question; it adds a second, catalog-time answer to a different
  question, on an already-existing endpoint. **No new endpoint was added** — the
  addition is a field on the existing catalog response, matching the "embed rather
  than add an endpoint" precedent this decision record already established for the
  per-tag field.
- **No public write endpoint for the policy flag itself.** Scoped to the write
  path, which is what this constraint actually protects: still no route writes
  `tag_purpose_policies`, and still no HTTP route calls
  `TagPurposePolicyServicer.Upsert`. `TagPurposePolicyServicer`'s access-control
  posture — no `Authorizer` call, no route behind it — is unchanged by the
  catalog-exposure addendum above. Worth stating explicitly, since it is the
  subtle part: the catalog-time `oneOfDomain` flag on `GET /tag-templates` reaches
  clients via the SQL join in `ListTagTemplates` described above, not by any route
  calling `TagPurposePolicyServicer.Get` — no HTTP route calls `Get` either, so
  that servicer genuinely still has no caller behind an HTTP route
  (`api/service/tag_purpose_policy.go` is untouched by the addendum). This is a
  deliberate choice, not an oversight: `one_of_domain` is a curated, admin-set
  value, not something exposed for end-user or public API writes. Values are
  seeded/managed out-of-band (direct SQL, a future admin surface, or a consuming
  app's own startup hook) until a curated admin surface is designed — see
  [`next-steps.md`](../../next-steps.md).

### GUI decisions

- **"The picker" is the existing purpose input, not a new widget.** `TagEditor`
  (`gui/src/TagEditor.tsx`) has no dedicated combo-search/autocomplete widget and no
  wiring to the `tag_templates` catalog. Its purpose-entry UI is one of: a
  free-form `<input>`, a fixed `<span>` (single-purpose mode), or a plain `<select>`
  populated from the host-app-supplied `purposes` prop (multi-purpose mode). This
  feature treats that existing UI as "the picker" and does not introduce a new
  combo-box/autocomplete widget wired to `tag_templates` — that would be
  substantial, unrelated net-new scope.
- **Fixing the pre-existing client-side duplicate-purpose validation gap.**
  `TagEditor.handleAddSubmit` previously performed no client-side purpose-conflict
  check at all — it validated only that `purpose`/`value` were non-empty and
  submitted directly, so a duplicate-purpose submission (e.g. `priority:low` then
  `priority:urgent`) depended entirely on the server round trip. This gap is fixed
  as part of this feature, since the new `one_of_domain`-aware exclusion logic
  requires building this exact client-side check anyway. `TagEditor` computes
  `occupiedPurposes` — the set of purposes among the subject's already-loaded tags
  where `oneOfDomain === true` — and uses it two ways: the multi-purpose `<select>`
  excludes any purpose already in `occupiedPurposes` from its rendered `<option>`
  list, and all entry modes (`select`, free-form, fixed) block submission
  client-side with a synthesized field error if the purpose to submit is in
  `occupiedPurposes`, instead of relying on the server round trip.

## Consequences

| Aspect | Status after this change |
|---|---|
| `tags_owner_subject_purpose_idx` | No longer a uniqueness guarantee by itself — it is a plain lookup index; `tags_enforce_one_of_domain` is now load-bearing for one-of-domain exclusivity. |
| `tag_purpose_policies` rows | Admin-managed only; no public write path. Seeded out-of-band until a curated admin surface is designed. |
| Advisory-lock mitigation | A documented, accepted tradeoff of the trigger-based approach vs. a true unique index — closes a TOCTOU gap a plain `EXISTS` check would otherwise have, at the cost of serializing concurrent inserts for the same `(owner_id, subject_id, purpose)` tuple. |
| Purposes with no policy row | Default to `one_of_domain = false` (multiple tags allowed) — matches this feature's explicit default and today's desired end-state behavior for ad hoc/unregistered purposes. |
| `GET /tag-templates` (catalog read) | Now a reader of `tag_purpose_policies`, via a `LEFT JOIN` in `ListTagTemplates` — the first non-`tags`-table consumer of the policy registry. Each row carries `oneOfDomain: boolean`, letting a client know a purpose is exclusive before any tag of it exists. No new route or write path; `TagPurposePolicyServicer` still has no HTTP-route caller. |
