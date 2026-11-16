-- @materialized: incremental
-- @incremental_strategy: delete+insert
-- @unique_key: id
SELECT id, amount FROM sqlforge_it_raw_events
