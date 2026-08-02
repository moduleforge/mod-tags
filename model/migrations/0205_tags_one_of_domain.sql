-- +goose Up

-- tag_purpose_policies is the per-purpose one_of_domain policy registry. It
-- is keyed purely on purpose (not scope) -- purpose semantics are global,
-- not per-app. It is distinct from tag_templates (an open, read-only
-- suggestion catalog for UI pickers, with no auth gating): this table backs
-- enforcement of the one-of-domain rule below. Absence of a row for a
-- purpose means one_of_domain = false (the default -- see the
-- tags_enforce_one_of_domain trigger's NOT FOUND fallback). Administered
-- out-of-band; no public write endpoint in this plan (a later phase adds an
-- internal-only servicer).
CREATE TABLE tag_purpose_policies (
  purpose        TEXT NOT NULL PRIMARY KEY CHECK (char_length(purpose) <= 512),
  one_of_domain  BOOLEAN NOT NULL DEFAULT false,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER tag_purpose_policies_set_updated_at
  BEFORE UPDATE ON tag_purpose_policies
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- The previous unconditional UNIQUE INDEX tags_owner_subject_purpose_idx is
-- replaced by a plain (non-unique) index of the same name and columns.
-- Conditional uniqueness driven by a lookup into tag_purpose_policies cannot
-- be expressed as a plain/partial index; enforcement moves to the
-- tags_enforce_one_of_domain trigger below. The index itself is retained --
-- only its uniqueness is removed -- so lookup performance for the trigger's
-- own check (and any other (owner_id, subject_id, purpose) query) is
-- unaffected.
DROP INDEX tags_owner_subject_purpose_idx;
CREATE INDEX tags_owner_subject_purpose_idx
  ON tags (owner_id, subject_id, purpose);

-- Enforce the one-of-domain rule: when tag_purpose_policies.one_of_domain is
-- true for NEW.purpose (defaulting to false when no policy row exists),
-- reject an insert that would create a second tag of that purpose for the
-- same (owner_id, subject_id). BEFORE INSERT only -- owner_id, subject_id,
-- and purpose are already immutable after insert
-- (tags_reject_immutable_changes), so no UPDATE path can create a new
-- conflict. pg_advisory_xact_lock closes the check-then-insert race a
-- trigger-based check has relative to a true unique index; do not drop it.
-- The exception is raised USING ERRCODE = 'unique_violation' (SQLSTATE
-- 23505) so api/service/tag.go's existing pgErr.Code == pgUniqueViolation
-- check already classifies this as ErrConflict (409) with zero Go changes.
-- +goose StatementBegin
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
-- +goose StatementEnd

CREATE TRIGGER tags_enforce_one_of_domain
  BEFORE INSERT ON tags
  FOR EACH ROW EXECUTE FUNCTION tags_enforce_one_of_domain();
