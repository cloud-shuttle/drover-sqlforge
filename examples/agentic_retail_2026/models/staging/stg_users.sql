-- @materialized: table
-- @grain: user_id

SELECT
    toString(number) AS user_id,
    now() - toIntervalDay(number) AS created_at,
    'US' AS country,
    multiIf(number % 2 == 0, 'premium', 'free') AS segment
FROM system.numbers
LIMIT 100
