-- Migration: backfill entities.owner_id for existing tag entities.
--
-- Cross-repo ordering precondition: this UPDATE assumes mod-core's
-- 0013_entity_ownership.sql (which adds entities.owner_id AND the
-- entities_owner_immutable trigger) has already run. In the composed
-- migration order, mod-core's 0013 sorts before mod-tags' 0203, so a fresh
-- `goose up` on the composed dir applies them in the correct order
-- automatically -- there is no window where 0013's trigger does not yet
-- exist when this migration runs.
--
-- Because the trigger already exists at this point, the naive
-- `UPDATE entities SET owner_id = ...` below is itself a NULL -> value write
-- of owner_id, which is exactly the transition entities_owner_immutable
-- guards against (BEFORE UPDATE OF owner_id, IF OLD.owner_id IS DISTINCT
-- FROM NEW.owner_id THEN RAISE EXCEPTION) -- it does not special-case a prior
-- NULL. This backfill is a legitimate one-time population of pre-existing
-- rows (not a runtime re-assignment of an already-owned entity), so the
-- trigger is disabled for the duration of this single statement only, then
-- immediately re-enabled. This preserves the immutability guarantee for all
-- other UPDATEs (including any subsequent attempt to change owner_id again)
-- while allowing this specific backfill to proceed.

-- +goose Up
ALTER TABLE entities DISABLE TRIGGER entities_owner_immutable;
UPDATE entities e SET owner_id = t.owner_id FROM tags t WHERE t.entity_id = e.id;
ALTER TABLE entities ENABLE TRIGGER entities_owner_immutable;

-- +goose Down
-- No-op: owner_id backfill is not reversible in isolation (the column and its
-- immutability trigger are owned by mod-core's 0013 migration).
