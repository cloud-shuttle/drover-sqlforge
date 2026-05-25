-- Singular SQL Test: Event amounts must always be non-negative (returns 0 rows on pass)
SELECT * FROM stg_events WHERE amount < 0;
