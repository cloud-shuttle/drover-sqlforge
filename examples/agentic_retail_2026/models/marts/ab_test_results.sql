-- @materialized: table

SELECT 
    x.experiment_id,
    x.variant,
    DATE_TRUNC('day', e.event_time) AS experiment_date,
    COUNT(DISTINCT e.user_id) AS unique_users,
    SUM(CASE WHEN e.event_type = 'purchase' THEN e.amount ELSE 0 END) AS revenue,
    SUM(CASE WHEN e.event_type = 'purchase' THEN e.amount ELSE 0 END) / NULLIF(COUNT(DISTINCT e.user_id), 0) AS revenue_per_user
FROM stg_events e
JOIN stg_experiments x 
    ON e.user_id = x.user_id
GROUP BY x.experiment_id, x.variant, experiment_date;
