# Tags Immutability Decision Record

## Purpose and scope

This decision record documents the immutability policy for the `tags` table: which
columns are immutable and why, why `value` is mutable, and how the original review
concern about value immutability (tracked as concern RF-3) was resolved. It is the
authoritative reference for the enforcement mechanism and the reasoning behind it.

## Context

The `tags` table stores a single-subject tag subtype. Each row is defined by an
identity tuple — `(owner_id, subject_id, purpose)` — that is enforced unique by the
[`tags_owner_subject_purpose_idx`](../../model/migrations/0201_tags.sql) index.
Alongside the identity columns, the table carries a `value` column (the data payload
the tag holds within that identity) and an optional `color` column.

During the initial design review, a `BEFORE UPDATE` trigger function
`tags_reject_immutable_changes` was added to prevent post-insert changes to
`owner_id`, `subject_id`, `purpose`, and `value`. The rationale for the first three
was clear — they form the identity — but `value` was included without explicit
justification.

Review concern RF-3 flagged this as a problem: treating `value` as immutable blocks
legitimate edit-in-place workflows (for example, a tag with purpose "status" whose
value moves from "draft" to "active"). RF-3 is an internal review-comment identifier
used during the planning phase; there is no external ticket URL.

## Decision

### The immutable identity tuple

`(owner_id, subject_id, purpose)` is the tag's identity and remains immutable after
insert. The rationale:

- The unique index `tags_owner_subject_purpose_idx` on `(owner_id, subject_id,
  purpose)` means this tuple uniquely identifies a tag row. Mutating any member of
  the tuple would either re-identify the tag (changing which logical tag a row
  represents) or collide with a different existing tag.
- The correct operation for changing an owner, subject, or purpose is delete the old
  tag and create a new one. This preserves the audit trail, avoids silent
  re-identification, and keeps the unique index semantics correct.

### Why `value` is mutable

`value` is the tag's *payload* — the data the tag carries within its fixed identity
— not a member of that identity. It is not part of the unique index, so editing it
cannot create a uniqueness collision or change which tag a row represents.

A common and valid workflow is an in-place update to a tag's value (e.g. purpose
"status" moving from "draft" to "active"). Blocking this with an immutability trigger
serves no integrity purpose and imposes unnecessary application complexity. `value`
is therefore freely updatable.

### Resolution of RF-3

The Phase 2 migration removes `value` from the `tags_reject_immutable_changes`
trigger. After the migration, only `owner_id`, `subject_id`, and `purpose` are
guarded by the trigger. The identity tuple stays immutable; the payload does not.

`color` was never included in the immutability trigger and remains freely updatable.

## Consequences

### Enforcement mechanism after resolution

The `BEFORE UPDATE` trigger function `tags_reject_immutable_changes` raises an
exception when any of `owner_id`, `subject_id`, or `purpose` changes on update.
`value` and `color` are not checked and can be updated freely. The trigger is
installed as:

```sql
CREATE TRIGGER tags_reject_immutable_changes
  BEFORE UPDATE ON tags
  FOR EACH ROW EXECUTE FUNCTION tags_reject_immutable_changes();
```

### Column mutability summary

| Column      | Mutable after insert | Reason |
|-------------|---------------------|--------|
| `owner_id`  | No | Part of the identity tuple and unique index |
| `subject_id`| No | Part of the identity tuple and unique index |
| `purpose`   | No | Part of the identity tuple and unique index |
| `value`     | Yes | Payload; not in the unique index; legitimate in-place updates expected |
| `color`     | Yes | Display attribute; not an identity field |

### Follow-on work

The Phase 2 database migration, Phase 3 API layer, and Phase 4 GUI layer add
end-to-end support for updating a tag's value. This document is the authorization
for that work.
