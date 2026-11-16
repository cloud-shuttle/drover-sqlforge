-- @materialized: incremental
-- @unique_key: id
SELECT id, amount, updated_at FROM sqlforge_it_raw_events
