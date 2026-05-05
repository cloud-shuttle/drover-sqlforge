-- @materialized: incremental
-- @incremental_strategy: auto
-- @unique_key: metric_date
-- @grain: metric_date

SELECT 
    DATE_TRUNC('day', event_time) AS metric_date,
    COUNT(DISTINCT user_id) AS daily_active_users,
    SUM(CASE WHEN event_type = 'purchase' THEN amount ELSE 0 END) AS daily_revenue,
    AVG(session_duration) AS avg_session_duration
FROM stg_events
GROUP BY metric_date;
