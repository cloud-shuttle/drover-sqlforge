-- @materialized: table
-- @test_not_null: id, amount
-- @test_unique: id
SELECT id, amount, updated_at FROM sqlforge_it_raw_events
