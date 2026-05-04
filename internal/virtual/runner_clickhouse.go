package virtual

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

type ClickHouseRunner struct {
	db   *sql.DB
	stub bool
}

func NewClickHouseRunner(dsn string) (*ClickHouseRunner, error) {
	if dsn == "" || dsn == "my_clickhouse" {
		return &ClickHouseRunner{stub: true}, nil
	}

	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	return &ClickHouseRunner{db: db, stub: false}, nil
}

func (r *ClickHouseRunner) Exec(ctx context.Context, query string) error {
	if r.stub {
		fmt.Printf("[ClickHouse Runner] Executing: %s\n", query)
		return nil
	}
	fmt.Printf("[ClickHouse Live] Executing: %s\n", query)
	_, err := r.db.ExecContext(ctx, query)
	return err
}

func (r *ClickHouseRunner) CreateSchemaDDL(schema string) string {
	return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", schema)
}

func (r *ClickHouseRunner) CreateTableDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE TABLE %s.%s ENGINE = MergeTree ORDER BY tuple() AS\n%s", schema, table, selectSQL)
}

func (r *ClickHouseRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE VIEW %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *ClickHouseRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE MATERIALIZED VIEW IF NOT EXISTS %s.%s ENGINE = MergeTree ORDER BY tuple() AS\n%s", schema, table, selectSQL)
}

func (r *ClickHouseRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	mat := config["_materialization_type"]
	if mat == "nats" {
		natsUrl := config["nats_url"]
		natsSubjects := config["nats_subjects"]
		natsFormat := config["nats_format"]
		if natsFormat == "" {
			natsFormat = "JSONEachRow"
		}
		return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
    raw_data String
) ENGINE = NATS
SETTINGS nats_url = '%s',
         nats_subjects = '%s',
         nats_format = '%s'`, schema, table, natsUrl, natsSubjects, natsFormat)
	}

	kafkaBroker := config["kafka_broker_list"]
	kafkaTopic := config["kafka_topic_list"]
	kafkaGroup := config["kafka_group_name"]
	kafkaFormat := config["kafka_format"]
	if kafkaFormat == "" {
		kafkaFormat = "JSONEachRow"
	}

	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
    raw_data String
) ENGINE = Kafka
SETTINGS kafka_broker_list = '%s',
         kafka_topic_list = '%s',
         kafka_group_name = '%s',
         kafka_format = '%s'`, schema, table, kafkaBroker, kafkaTopic, kafkaGroup, kafkaFormat)
}

func (r *ClickHouseRunner) Name() string {
	return "clickhouse"
}
