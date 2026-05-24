-- @materialized: table
-- @grain: metric_date, country, user_id

SELECT 
    DATE_TRUNC('day', e.event_time) AS metric_date,
    u.country AS country,
    e.user_id AS user_id,
    e.event_type AS event_type,
    e.amount AS amount,
    e.session_duration AS session_duration
FROM stg_events e
LEFT JOIN stg_users u ON e.user_id = u.user_id;
