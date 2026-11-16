package state

import "testing"

func TestGetOrCreateEnv_ValidatesName(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env, err := m.GetOrCreateEnv("peter_dev", "prod")
	if err != nil || env.Schema != "sqlforge__peter_dev" {
		t.Fatalf("env = %+v, err = %v", env, err)
	}
	for _, bad := range []string{"", "dev;DROP SCHEMA prod CASCADE", "dev-1", "1dev", "_x", "dev x"} {
		if _, err := m.GetOrCreateEnv(bad, "prod"); err == nil {
			t.Errorf("%q: want error", bad)
		}
		if _, err := m.Store.GetEnvironment(bad); err == nil {
			t.Errorf("%q: environment was persisted", bad)
		}
	}
}
