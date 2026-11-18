-- @materialized: incremental
-- @incremental_strategy: merge
-- @unique_key: id
SELECT 1 AS id, 'm1' AS name
UNION ALL
SELECT 3 AS id, 'm3' AS name;
