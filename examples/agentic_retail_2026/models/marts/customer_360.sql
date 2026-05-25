-- @materialized: table
-- @environment: virtual
-- @grain: user_id
-- @description: Unified customer profile with behavioral features for AI agents
-- @test_relationship: user_id to stg_users.user_id

SELECT 
    u.user_id,
    u.created_at,
    u.country,
    u.segment,
    
    -- Behavioral aggregates (last 30/90 days)
    COUNT(CASE WHEN e.event_time >= NOW() - INTERVAL '30 days' THEN 1 END) AS events_last_30d,
    SUM(CASE WHEN e.event_type = 'purchase' THEN e.amount ELSE 0 END) AS revenue_last_90d,
    
    -- Simple embedding-style features (for agentic use)
    AVG(e.session_duration) AS avg_session_duration,
    COUNT(DISTINCT DATE_TRUNC('week', e.event_time)) AS active_weeks_last_90d,

    -- Readiness for agents / feature store
    NOW() AS feature_updated_at
FROM stg_events e
LEFT JOIN stg_users u ON e.user_id = u.user_id
WHERE e.event_time >= today() - INTERVAL '90 days'
GROUP BY u.user_id, u.created_at, u.country, u.segment;
