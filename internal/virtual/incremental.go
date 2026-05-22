package virtual

import (
	"fmt"
	"strings"
)

// Incremental strategy names (from model config incremental_strategy).
const (
	StrategyAuto               = "auto"
	StrategyAppend             = "append"
	StrategyMerge              = "merge"
	StrategyUpsert             = "upsert"
	StrategyDeleteInsert       = "delete+insert"
	StrategyReplacingMergeTree = "replacing_merge_tree"
)

// ResolveIncrementalStrategy returns the effective strategy after applying defaults.
func ResolveIncrementalStrategy(config map[string]string) string {
	raw := strings.ToLower(strings.TrimSpace(config["incremental_strategy"]))
	switch raw {
	case "", StrategyAuto:
		if strings.TrimSpace(config["unique_key"]) != "" {
			return StrategyMerge
		}
		return StrategyAppend
	case StrategyUpsert:
		return StrategyMerge
	case "delete_insert":
		return StrategyDeleteInsert
	default:
		return raw
	}
}

// BuildIncrementalMergeDDL generates dialect-specific incremental load SQL.
func BuildIncrementalMergeDDL(dialect, schema, table, selectSQL string, config map[string]string) (string, error) {
	strategy := ResolveIncrementalStrategy(config)
	qualified := fmt.Sprintf("%s.%s", schema, table)
	uniqueKey := strings.TrimSpace(config["unique_key"])

	switch strategy {
	case StrategyAppend:
		return fmt.Sprintf("INSERT INTO %s\nSELECT * FROM (%s);", qualified, selectSQL), nil
	case StrategyMerge:
		if uniqueKey == "" {
			return "", fmt.Errorf("incremental strategy merge requires unique_key")
		}
		return buildMergeDDL(dialect, qualified, selectSQL, uniqueKey)
	case StrategyDeleteInsert:
		if uniqueKey == "" {
			return "", fmt.Errorf("incremental strategy delete+insert requires unique_key")
		}
		return fmt.Sprintf(
			"DELETE FROM %s WHERE %s IN (SELECT %s FROM (%s));\nINSERT INTO %s\nSELECT * FROM (%s);",
			qualified, uniqueKey, uniqueKey, selectSQL, qualified, selectSQL,
		), nil
	case StrategyReplacingMergeTree:
		// Incremental step for ClickHouse ReplacingMergeTree is append; dedup is engine-side.
		return fmt.Sprintf("INSERT INTO %s\nSELECT * FROM (%s);", qualified, selectSQL), nil
	default:
		return "", fmt.Errorf("unknown incremental_strategy %q", strategy)
	}
}

func buildMergeDDL(dialect, qualified, selectSQL, uniqueKey string) (string, error) {
	switch strings.ToLower(dialect) {
	case "snowflake", "databricks", "velodb":
		return fmt.Sprintf(
			"MERGE INTO %s t\nUSING (%s) s\nON t.%s = s.%s\nWHEN MATCHED THEN UPDATE SET *\nWHEN NOT MATCHED THEN INSERT *;",
			qualified, selectSQL, uniqueKey, uniqueKey,
		), nil
	case "doris":
		return fmt.Sprintf(
			"INSERT INTO %s\nSELECT * FROM (%s)\nON DUPLICATE KEY UPDATE *;",
			qualified, selectSQL,
		), nil
	case "clickhouse":
		return fmt.Sprintf("INSERT INTO %s\nSELECT * FROM (%s);", qualified, selectSQL), nil
	default:
		// duckdb, postgres, and other ANSI engines
		return fmt.Sprintf(
			"INSERT INTO %s\nSELECT * FROM (%s)\nON CONFLICT (%s) DO UPDATE SET *;",
			qualified, selectSQL, uniqueKey,
		), nil
	}
}

// BuildIncrementalInitialDDL creates the target table on first incremental apply.
func BuildIncrementalInitialDDL(dialect, schema, table, selectSQL string, config map[string]string) (string, error) {
	strategy := ResolveIncrementalStrategy(config)
	qualified := fmt.Sprintf("%s.%s", schema, table)

	if strategy == StrategyReplacingMergeTree && strings.ToLower(dialect) == "clickhouse" {
		versionCol := strings.TrimSpace(config["updated_at"])
		if versionCol == "" {
			versionCol = "updated_at"
		}
		orderBy := strings.TrimSpace(config["unique_key"])
		if orderBy == "" {
			orderBy = "tuple()"
		}
		return fmt.Sprintf(
			"CREATE OR REPLACE TABLE %s ENGINE = ReplacingMergeTree(%s) ORDER BY (%s) AS\n%s",
			qualified, versionCol, orderBy, selectSQL,
		), nil
	}

	switch strings.ToLower(dialect) {
	case "clickhouse":
		return fmt.Sprintf("CREATE OR REPLACE TABLE %s ENGINE = MergeTree ORDER BY tuple() AS\n%s", qualified, selectSQL), nil
	case "postgres":
		return fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;\nCREATE TABLE %s AS\n%s", qualified, qualified, selectSQL), nil
	default:
		return fmt.Sprintf("CREATE OR REPLACE TABLE %s AS\n%s", qualified, selectSQL), nil
	}
}
