package integration

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

func TestClickHouseIntegration(t *testing.T) {
	// 1. Ensure ClickHouse is running
	db, err := sql.Open("clickhouse", "clickhouse://default:@localhost:9000/default")
	if err != nil {
		t.Fatalf("Failed to open clickhouse connection: %v", err)
	}
	defer db.Close()

	// Wait up to 10 seconds for DB to be ready
	ready := false
	for i := 0; i < 10; i++ {
		if err := db.Ping(); err == nil {
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !ready {
		t.Skip("ClickHouse is not running on localhost:9000. Skipping integration test. Run 'make integration' to test via Docker Compose.")
	}

	// 2. Setup paths
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "..", "..")
	cliPath := filepath.Join(projectRoot, "sqlforge")
	projectDir := filepath.Join(cwd, "project")

	if _, err := os.Stat(cliPath); os.IsNotExist(err) {
		t.Fatalf("sqlforge binary not found at %s. Please run 'make e2e' to build the binary before testing.", cliPath)
	}

	// Clean previous state
	stateDir := filepath.Join(projectDir, ".sqlforge")
	os.RemoveAll(stateDir)
	defer os.RemoveAll(stateDir)

	// Clean database state
	_, _ = db.Exec("DROP DATABASE IF EXISTS sqlforge__prod")

	// Helper to run CLI
	runCLI := func(args ...string) (string, error) {
		cmd := exec.Command(cliPath, args...)
		cmd.Dir = projectDir
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		return out.String(), err
	}

	// 3. Run plan & apply
	_, err = runCLI("plan", "prod")
	if err != nil {
		t.Fatalf("Failed to run plan prod: %v", err)
	}

	out, err := runCLI("apply", "prod")
	if err != nil {
		t.Fatalf("Failed to run apply prod: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(out, "Apply completed successfully.") {
		t.Errorf("Expected apply output to indicate success, got: %s", out)
	}

	// 4. Verify in live ClickHouse database!
	var count int
	err = db.QueryRow("SELECT count(*) FROM sqlforge__prod.stg_users").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query live database table stg_users: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 rows in stg_users, got %d", count)
	}

	var name string
	err = db.QueryRow("SELECT name FROM sqlforge__prod.users_view WHERE id = 1").Scan(&name)
	if err != nil {
		t.Fatalf("Failed to query live database view users_view: %v", err)
	}
	if name != "alice" {
		t.Errorf("Expected 'alice' in users_view, got '%s'", name)
	}

	fmt.Println("Successfully verified generated DDL and Data Quality against Live ClickHouse!")
}
