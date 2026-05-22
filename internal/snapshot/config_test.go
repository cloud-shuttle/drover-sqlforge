package snapshot

import "testing"

func TestResolveConfigGrainFallback(t *testing.T) {
	cfg, err := ResolveConfig(&Definition{
		Name: "users",
		Config: map[string]string{
			"grain":       "user_id",
			"updated_at":  "created_at",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UniqueKey != "user_id" {
		t.Errorf("expected grain fallback, got %q", cfg.UniqueKey)
	}
}
