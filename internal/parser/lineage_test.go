package parser

import (
	"context"
	"strings"
	"testing"
)

func TestExtractColumnLineage_joinedModel(t *testing.T) {
	sql := `
SELECT
    u.user_id,
    u.country,
    COUNT(CASE WHEN e.event_time >= NOW() - INTERVAL '30 days' THEN 1 END) AS events_last_30d,
    SUM(CASE WHEN e.event_type = 'purchase' THEN e.amount ELSE 0 END) AS revenue_last_90d
FROM stg_events e
LEFT JOIN stg_users u ON e.user_id = u.user_id
GROUP BY u.user_id, u.country`

	p, err := NewParser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	mappings, err := p.ExtractColumnLineage(sql)
	if err != nil {
		t.Fatal(err)
	}
	if len(mappings) < 4 {
		t.Fatalf("expected at least 4 mappings, got %d", len(mappings))
	}

	byOutput := map[string]ColumnMapping{}
	for _, m := range mappings {
		byOutput[m.Output] = m
	}

	assertHasSource(t, byOutput["user_id"], "stg_users", "user_id")
	assertHasSource(t, byOutput["country"], "stg_users", "country")
	assertHasSource(t, byOutput["events_last_30d"], "stg_events", "event_time")
	assertHasSource(t, byOutput["revenue_last_90d"], "stg_events", "event_type")
	assertHasSource(t, byOutput["revenue_last_90d"], "stg_events", "amount")
}

func TestExtractColumnLineage_simpleSelect(t *testing.T) {
	sql := `SELECT toString(number) AS user_id, 'US' AS country FROM system.numbers`
	p, err := NewParser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	mappings, err := p.ExtractColumnLineage(sql)
	if err != nil {
		t.Fatal(err)
	}
	if len(mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(mappings))
	}
	if mappings[0].Output != "user_id" || len(mappings[0].Sources) != 0 {
		t.Errorf("user_id mapping: %+v", mappings[0])
	}
	if mappings[1].Output != "country" {
		t.Errorf("country output: %s", mappings[1].Output)
	}
}

func assertHasSource(t *testing.T, m ColumnMapping, relation, column string) {
	t.Helper()
	for _, s := range m.Sources {
		if s.Relation == relation && s.Column == column {
			return
		}
	}
	var got []string
	for _, s := range m.Sources {
		got = append(got, s.Relation+"."+s.Column)
	}
	t.Fatalf("output %q missing %s.%s; sources: %s", m.Output, relation, column, strings.Join(got, ", "))
}
