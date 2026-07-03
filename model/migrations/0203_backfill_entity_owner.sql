-- Migration: backfill entities.owner_id for existing tag entities.
--
-- Cross-repo ordering precondition: this UPDATE assumes mod-core's
-- 0013_entity_ownership.sql (which adds entities.owner_id) has already run.
-- In the composed migration order, mod-core's 0013 sorts before mod-tags'
-- 0203, so a fresh `goose up` on the composed dir applies them in the
-- correct order automatically.
--
-- entities.owner_id is immutable-after-insert (mod-core 0013's
-- entities_owner_immutable trigger), so this backfill must be the first
-- write of the value for each tag entity -- which it is, since existing
-- tag entities have owner_id = NULL (tag types do not descend from
-- natural_person/service_account, so the owns-itself default does not
-- fire for them).

-- +goose Up
UPDATE entities e SET owner_id = t.owner_id FROM tags t WHERE t.entity_id = e.id;

-- +goose Down
-- No-op: owner_id backfill is not reversible in isolation (the column and its
-- immutability trigger are owned by mod-core's 0013 migration).
