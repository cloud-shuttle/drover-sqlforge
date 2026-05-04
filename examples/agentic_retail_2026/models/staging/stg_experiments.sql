-- @materialized: table
-- @grain: experiment_id

SELECT
    'exp_2026' AS experiment_id,
    toString(number) AS user_id,
    multiIf(number % 2 == 0, 'control', 'treatment') AS variant,
    now() - toIntervalDay(number) AS assigned_at
FROM system.numbers
LIMIT 100
