-- +goose Up
CREATE TABLE tag_templates (
  id          BIGSERIAL PRIMARY KEY,
  scope       BIGINT REFERENCES apps(id) ON DELETE CASCADE,       -- NULL = global template
  purpose     TEXT NOT NULL CHECK (char_length(purpose) <= 512),
  value       TEXT NOT NULL CHECK (char_length(value) <= 512),
  label       TEXT NOT NULL CHECK (char_length(label) <= 512),
  color       TEXT CHECK (color SIMILAR TO '#[0-9A-Fa-f]{8}'),
  sort_order  INTEGER NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One scoped row per (scope, purpose, value).
CREATE UNIQUE INDEX tag_templates_scoped_purpose_value_idx
  ON tag_templates (scope, purpose, value) WHERE scope IS NOT NULL;

-- One GLOBAL row per (purpose, value). A plain UNIQUE (scope, purpose, value)
-- would NOT dedupe global rows because NULL != NULL, so global uniqueness needs
-- its own partial index over the scope-IS-NULL subset.
CREATE UNIQUE INDEX tag_templates_global_purpose_value_idx
  ON tag_templates (purpose, value) WHERE scope IS NULL;

-- Supports the list query's (purpose [, scope]) filter.
CREATE INDEX tag_templates_purpose_scope_idx ON tag_templates (purpose, scope);

CREATE TRIGGER tag_templates_set_updated_at
  BEFORE UPDATE ON tag_templates
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS tag_templates;
