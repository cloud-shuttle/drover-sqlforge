-- @materialized: table
-- @grain: user_id
-- @test_not_null: user_id, country
-- @test_unique: user_id
-- @test_accepted_values_segment: premium, free

SELECT
    toString(number) AS user_id,
    now() - toIntervalDay(number) AS created_at,
    'US' AS country,
    multiIf(number % 2 == 0, 'premium', 'free') AS segment
FROM system.numbers
LIMIT 100
