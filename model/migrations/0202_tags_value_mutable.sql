-- Migration: remove `value` from the tags immutability policy.
--
-- `value` is payload — it describes what the tag means, not which tag it is.
-- Identity columns (`owner_id`, `subject_id`, `purpose`) remain immutable
-- because they form the unique key of a tag relationship and changing them
-- would silently move the tag to a different relationship.
--
-- See docs/decisions/tags-immutability.md for the full rationale.

-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION tags_reject_immutable_changes() RETURNS TRIGGER AS $$
BEGIN
  IF OLD.owner_id IS DISTINCT FROM NEW.owner_id THEN
    RAISE EXCEPTION 'tags: owner_id is immutable after insert';
  END IF;
  IF OLD.subject_id IS DISTINCT FROM NEW.subject_id THEN
    RAISE EXCEPTION 'tags: subject_id is immutable after insert';
  END IF;
  IF OLD.purpose IS DISTINCT FROM NEW.purpose THEN
    RAISE EXCEPTION 'tags: purpose is immutable after insert';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down

-- Restore original function body that also treats `value` as immutable.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION tags_reject_immutable_changes() RETURNS TRIGGER AS $$
BEGIN
  IF OLD.owner_id IS DISTINCT FROM NEW.owner_id THEN
    RAISE EXCEPTION 'tags: owner_id is immutable after insert';
  END IF;
  IF OLD.subject_id IS DISTINCT FROM NEW.subject_id THEN
    RAISE EXCEPTION 'tags: subject_id is immutable after insert';
  END IF;
  IF OLD.purpose IS DISTINCT FROM NEW.purpose THEN
    RAISE EXCEPTION 'tags: purpose is immutable after insert';
  END IF;
  IF OLD.value IS DISTINCT FROM NEW.value THEN
    RAISE EXCEPTION 'tags: value is immutable after insert';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
