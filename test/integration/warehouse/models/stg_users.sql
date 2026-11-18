-- @materialized: table
SELECT 1 AS id, 'alice' AS name, 'keep;me' AS note
UNION ALL
SELECT 2 AS id, 'bob' AS name, 'plain' AS note;
