-- @materialized: incremental
-- @incremental_strategy: append
SELECT 1 AS id, 'inc' AS name
UNION ALL
SELECT 2 AS id, 'inc' AS name;
