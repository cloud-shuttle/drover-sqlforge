-- @strategy: timestamp
-- @unique_key: user_id
-- @updated_at: created_at
-- @grain: user_id

SELECT
    toString(number) AS user_id,
    now() - toIntervalDay(number) AS created_at,
    'US' AS country,
    if(number % 2 = 0, 'premium', 'free') AS segment
FROM system.numbers
LIMIT 50
