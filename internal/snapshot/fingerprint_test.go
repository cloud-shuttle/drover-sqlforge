package snapshot

import "testing"

func TestFingerprintIgnoresKeyOrder(t *testing.T) {
	a := Fingerprint("SELECT 1", map[string]string{"strategy": "timestamp", "unique_key": "id"})
	b := Fingerprint("SELECT 1", map[string]string{"unique_key": "id", "strategy": "timestamp"})
	if a != b {
		t.Fatalf("order changed the hash: %s vs %s", a, b)
	}
	if a == Fingerprint("SELECT 2", map[string]string{"strategy": "timestamp", "unique_key": "id"}) {
		t.Fatal("different SQL hashed the same")
	}
	if a == Fingerprint("SELECT 1", map[string]string{"strategy": "check", "unique_key": "id"}) {
		t.Fatal("different config hashed the same")
	}
	if Fingerprint("SELECT 1", nil) == "" {
		t.Fatal("empty config still has a hash")
	}
}
