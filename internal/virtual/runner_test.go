package virtual

import (
	"strings"
	"testing"
)

func TestClickHouseStreamingDDL_Kafka(t *testing.T) {
	runner, err := NewRunner("clickhouse", "my_clickhouse")
	if err != nil {
		t.Fatalf("Failed to create clickhouse runner: %v", err)
	}

	config := map[string]string{
		"_materialization_type": "kafka",
		"kafka_broker_list":     "localhost:9092",
		"kafka_topic_list":      "my_topic",
		"kafka_group_name":      "my_group",
		"kafka_format":          "JSONAsString",
	}

	ddl := runner.CreateStreamingTableDDL("public", "events", config)

	if !strings.Contains(ddl, "ENGINE = Kafka") {
		t.Errorf("Expected Kafka engine, got: %s", ddl)
	}
	if !strings.Contains(ddl, "kafka_broker_list = 'localhost:9092'") {
		t.Errorf("Missing broker list in settings")
	}
	if !strings.Contains(ddl, "kafka_format = 'JSONAsString'") {
		t.Errorf("Missing kafka format")
	}
}

func TestClickHouseStreamingDDL_Nats(t *testing.T) {
	runner, err := NewRunner("clickhouse", "my_clickhouse")
	if err != nil {
		t.Fatalf("Failed to create clickhouse runner: %v", err)
	}

	config := map[string]string{
		"_materialization_type": "nats",
		"nats_url":              "nats://localhost:4222",
		"nats_subjects":         "my_subject",
		"nats_format":           "JSONEachRow",
	}

	ddl := runner.CreateStreamingTableDDL("public", "events_nats", config)

	if !strings.Contains(ddl, "ENGINE = NATS") {
		t.Errorf("Expected NATS engine, got: %s", ddl)
	}
	if !strings.Contains(ddl, "nats_url = 'nats://localhost:4222'") {
		t.Errorf("Missing nats url in settings")
	}
	if !strings.Contains(ddl, "nats_subjects = 'my_subject'") {
		t.Errorf("Missing nats subject")
	}
}

func TestOtherRunnersFallback(t *testing.T) {
	dialects := []string{"duckdb", "postgres", "snowflake", "databricks", "doris", "velodb"}

	for _, dialect := range dialects {
		runner, err := NewRunner(dialect, "")
		if err != nil {
			t.Fatalf("Failed to create runner for %s: %v", dialect, err)
		}

		ddl := runner.CreateStreamingTableDDL("public", "test_table", nil)
		if !strings.Contains(ddl, "--") {
			t.Errorf("Expected fallback comment for %s, got: %s", dialect, ddl)
		}
	}
}

func FuzzClickHouseStreamingDDL(f *testing.F) {
	runner, err := NewRunner("clickhouse", "my_clickhouse")
	if err != nil {
		f.Fatalf("Failed to create clickhouse runner: %v", err)
	}

	f.Add("kafka", "my_broker", "my_topic", "my_group", "JSONAsString")
	f.Add("nats", "nats://localhost", "my_subject", "my_group", "JSONEachRow")
	f.Add("streaming", "", "", "", "")

	f.Fuzz(func(t *testing.T, mat, p1, p2, p3, p4 string) {
		config := map[string]string{
			"_materialization_type": mat,
			"kafka_broker_list":     p1,
			"kafka_topic_list":      p2,
			"kafka_group_name":      p3,
			"kafka_format":          p4,
			"nats_url":              p1,
			"nats_subjects":         p2,
			"nats_format":           p4,
		}

		// The goal of this fuzz test is simply to verify that DDL generation
		// never panics under random payload strings
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("CreateStreamingTableDDL panicked on inputs: mat=%s, p1=%s, p2=%s, p3=%s, p4=%s", mat, p1, p2, p3, p4)
			}
		}()

		_ = runner.CreateStreamingTableDDL("public", "fuzz_table", config)
	})
}
