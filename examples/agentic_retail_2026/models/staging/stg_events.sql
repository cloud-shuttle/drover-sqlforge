-- @materialized: incremental
-- @incremental_strategy: auto
-- @grain: event_id

SELECT
    toString(number) AS event_id,
    toString(number % 100) AS user_id,
    now() - toIntervalMinute(number) AS event_time,
    multiIf(number % 5 == 0, 'purchase', 'page_view') AS event_type,
    '/home' AS page_url,
    (number % 50) + 10.50 AS amount,
    (number % 300) AS session_duration
FROM system.numbers
LIMIT 1000
