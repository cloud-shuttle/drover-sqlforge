-- @materialized: incremental
-- @grain: (user_id, session_date)

SELECT 
    user_id,
    DATE_TRUNC('day', event_time) AS session_date,
    COUNT(DISTINCT event_id) AS events_in_session,
    COUNT(CASE WHEN event_type = 'purchase' THEN 1 END) AS purchases,
    MAX(event_time) AS last_event_time
FROM stg_events
GROUP BY user_id, session_date
