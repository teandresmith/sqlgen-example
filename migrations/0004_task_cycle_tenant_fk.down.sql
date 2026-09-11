-- Reverse 0004 in the opposite order: restore the single-column FK on
-- tasks.cycle_id, then drop the composite FK and the unique key it targeted.
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_cycle_same_ws;

ALTER TABLE tasks ADD CONSTRAINT tasks_cycle_id_fkey
    FOREIGN KEY (cycle_id) REFERENCES cycles (id);

ALTER TABLE cycles DROP CONSTRAINT IF EXISTS cycles_ws_id_key;
