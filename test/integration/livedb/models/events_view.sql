-- @materialized: view
SELECT id, amount * 2 AS doubled FROM stg_events
