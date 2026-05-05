package model

import (
	"strings"
	"testing"
)

func FuzzParseConfigLine(f *testing.F) {
	// Seed corpus
	f.Add("-- @materialized: table")
	f.Add("-- @materialized: table -- some comment")
	f.Add("-- @key: value:with:colons")
	f.Add("-- @:")
	f.Add("just some normal sql")
	f.Add("-- @ ")

	f.Fuzz(func(t *testing.T, line string) {
		key, value, ok := ParseConfigLine(line)

		if !ok {
			// If not OK, the strings should be empty
			if key != "" || value != "" {
				t.Errorf("Expected empty key and value when not ok, got key=%q, value=%q", key, value)
			}
			return
		}

		// If OK, the line must have started with -- @
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "-- @") {
			t.Errorf("Reported ok=true but didn't start with -- @: %q", line)
		}

		// Value should not contain trailing inline comments if properly removed
		if strings.Contains(value, "--") {
			t.Errorf("Value still contains comment marker: %q", value)
		}
	})
}
