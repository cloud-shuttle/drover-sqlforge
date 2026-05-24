package integration

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/clickhouse"
	_ "github.com/ClickHouse/clickhouse-go/v2"
)

// copyDir recursively copies a directory tree, attempting to preserve permissions.
func copyDir(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath, info.Mode())
	})
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func TestClickHouseIntegration(t *testing.T) {
	ctx := context.Background()

	// 1. Spin up ClickHouse Testcontainer
	clickhouseContainer, err := clickhouse.Run(ctx, "clickhouse/clickhouse-server:latest",
		clickhouse.WithUsername("default"),
		clickhouse.WithPassword("mysecretpassword"),
		clickhouse.WithDatabase("default"),
	)
	if err != nil {
		t.Fatalf("Failed to start clickhouse container: %v", err)
	}
	defer func() {
		if err := clickhouseContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	host, err := clickhouseContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}
	
	port, err := clickhouseContainer.MappedPort(ctx, "9000/tcp")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}
	
	connectionStr := fmt.Sprintf("clickhouse://default:mysecretpassword@%s:%s/default", host, port.Port())

	db, err := sql.Open("clickhouse", connectionStr)
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
		} else {
			t.Logf("Ping error: %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	if !ready {
		t.Fatalf("ClickHouse testcontainer did not become ready")
	}

	// 2. Setup paths
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "..", "..")
	cliPath := filepath.Join(projectRoot, "sqlforge")
	
	if _, err := os.Stat(cliPath); os.IsNotExist(err) {
		t.Fatalf("sqlforge binary not found at %s. Please run 'make e2e' to build the binary before testing.", cliPath)
	}

	// Copy project to temp dir
	tmpDir, err := os.MkdirTemp("", "sqlforge-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcProjectDir := filepath.Join(cwd, "project")
	projectDir := filepath.Join(tmpDir, "project")
	if err := copyDir(srcProjectDir, projectDir); err != nil {
		t.Fatalf("Failed to copy project dir: %v", err)
	}

	// Rewrite connection string in yaml
	yamlPath := filepath.Join(projectDir, "sqlforge.yml")
	yamlData, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("Failed to read sqlforge.yml: %v", err)
	}
	
	newYamlData := strings.ReplaceAll(string(yamlData), "clickhouse://default:@localhost:9000/default", connectionStr)
	if err := os.WriteFile(yamlPath, []byte(newYamlData), 0644); err != nil {
		t.Fatalf("Failed to write updated sqlforge.yml: %v", err)
	}

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

	fmt.Println("Successfully verified generated DDL and Data Quality against Live ClickHouse via Testcontainers!")
}
