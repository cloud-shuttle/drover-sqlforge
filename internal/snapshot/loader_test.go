package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSnapshotsMissingDir(t *testing.T) {
	defs, err := LoadSnapshots(filepath.Join(t.TempDir(), "missing"))
	if err != nil || defs != nil {
		t.Fatalf("missing dir: defs=%v err=%v", defs, err)
	}
}

func TestLoadSnapshotsParsesSQLAndSkipsOtherFiles(t *testing.T) {
	dir := t.TempDir()
	body := "" +
		"-- @unique_key: id\n" +
		"-- @strategy: timestamp\n" +
		"-- @updated_at: updated_at\n" +
		"SELECT 1 AS id\n"
	if err := os.WriteFile(filepath.Join(dir, "users.sql"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}

	defs, err := LoadSnapshots(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 {
		t.Fatalf("got %d definitions", len(defs))
	}
	got := defs[0]
	if got.Name != "users" {
		t.Fatalf("name %q", got.Name)
	}
	if got.Config["unique_key"] != "id" || got.Config["strategy"] != "timestamp" || got.Config["updated_at"] != "updated_at" {
		t.Fatalf("config %#v", got.Config)
	}
	if !strings.Contains(got.SQL, "SELECT 1 AS id") {
		t.Fatalf("sql %q", got.SQL)
	}
}

func TestFilterByNames(t *testing.T) {
	defs := []*Definition{{Name: "users"}, {Name: "orders"}}
	all, err := FilterByNames(defs, nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("empty names: %v %v", all, err)
	}
	one, err := FilterByNames(defs, []string{"orders"})
	if err != nil || len(one) != 1 || one[0].Name != "orders" {
		t.Fatalf("filter: %#v %v", one, err)
	}
	if _, err := FilterByNames(defs, []string{"missing"}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing name: %v", err)
	}
}
