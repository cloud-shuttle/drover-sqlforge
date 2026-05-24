package snapshot

import (
	"fmt"
	"strings"
)

// BuildRun returns ordered SQL statements for one snapshot execution.
func BuildRun(dialect, schema, table string, exists bool, sourceSQL string, cfg ResolvedConfig) ([]string, error) {
	qualified := fmt.Sprintf("%s.%s", schema, table)
	staging := fmt.Sprintf("sqlforge_snapshot_staging_%s", table)

	switch cfg.Strategy {
	case "timestamp":
		if !exists {
			return []string{initialBuildSQL(qualified, sourceSQL)}, nil
		}
		switch strings.ToLower(dialect) {
		case "clickhouse":
			return clickhouseTimestampRun(qualified, sourceSQL, cfg), nil
		default:
			return ansiTimestampRun(qualified, staging, sourceSQL, cfg), nil
		}
	case "check":
		if !exists {
			return []string{initialBuildSQL(qualified, sourceSQL)}, nil
		}
		switch strings.ToLower(dialect) {
		case "clickhouse":
			return clickhouseCheckRun(qualified, sourceSQL, cfg), nil
		default:
			return ansiCheckRun(qualified, staging, sourceSQL, cfg), nil
		}
	default:
		return nil, fmt.Errorf("unsupported strategy %q", cfg.Strategy)
	}
}

func initialBuildSQL(qualified, sourceSQL string) string {
	return fmt.Sprintf(
		"CREATE TABLE %s AS\nSELECT s.*,\n  CURRENT_TIMESTAMP AS %s,\n  CAST(NULL AS TIMESTAMP) AS %s\nFROM (\n%s\n) AS s",
		qualified, ValidFrom, ValidTo, sourceSQL,
	)
}

func ansiTimestampRun(qualified, staging, sourceSQL string, cfg ResolvedConfig) []string {
	uk := cfg.UniqueKey
	updatedAt := cfg.UpdatedAt
	return []string{
		fmt.Sprintf("CREATE OR REPLACE TEMP TABLE %s AS\nSELECT * FROM (\n%s\n) AS s", staging, sourceSQL),
		fmt.Sprintf(
			"UPDATE %s AS t\nSET %s = CURRENT_TIMESTAMP\nFROM %s AS s\nWHERE t.%s = s.%s\n  AND t.%s IS NULL\n  AND s.%s > t.%s",
			qualified, ValidTo, staging, uk, uk, ValidTo, updatedAt, updatedAt,
		),
		fmt.Sprintf(
			"INSERT INTO %s\nSELECT s.*, CURRENT_TIMESTAMP AS %s, CAST(NULL AS TIMESTAMP) AS %s\nFROM %s AS s\nLEFT JOIN %s AS t ON s.%s = t.%s AND t.%s IS NULL\nWHERE t.%s IS NULL OR s.%s > t.%s",
			qualified, ValidFrom, ValidTo, staging, qualified, uk, uk, ValidTo, uk, updatedAt, updatedAt,
		),
		fmt.Sprintf("DROP TABLE IF EXISTS %s", staging),
	}
}

func clickhouseTimestampRun(qualified, sourceSQL string, cfg ResolvedConfig) []string {
	// ClickHouse: append new versions; rely on query-time filtering or future ReplacingMergeTree option.
	_ = cfg
	return []string{
		fmt.Sprintf(
			"INSERT INTO %s\nSELECT s.*, now() AS %s, CAST(NULL AS Nullable(DateTime)) AS %s\nFROM (\n%s\n) AS s",
			qualified, ValidFrom, ValidTo, sourceSQL,
		),
	}
}

func ansiCheckRun(qualified, staging, sourceSQL string, cfg ResolvedConfig) []string {
	uk := cfg.UniqueKey
	var conditionBuilder strings.Builder
	for i, c := range cfg.CheckCols {
		if i > 0 {
			conditionBuilder.WriteString(" OR ")
		}
		conditionBuilder.WriteString(fmt.Sprintf("s.%s IS DISTINCT FROM t.%s", c, c))
	}
	checkCondition := conditionBuilder.String()

	return []string{
		fmt.Sprintf("CREATE OR REPLACE TEMP TABLE %s AS\nSELECT * FROM (\n%s\n) AS s", staging, sourceSQL),
		fmt.Sprintf(
			"UPDATE %s AS t\nSET %s = CURRENT_TIMESTAMP\nFROM %s AS s\nWHERE t.%s = s.%s\n  AND t.%s IS NULL\n  AND (%s)",
			qualified, ValidTo, staging, uk, uk, ValidTo, checkCondition,
		),
		fmt.Sprintf(
			"INSERT INTO %s\nSELECT s.*, CURRENT_TIMESTAMP AS %s, CAST(NULL AS TIMESTAMP) AS %s\nFROM %s AS s\nLEFT JOIN %s AS t ON s.%s = t.%s AND t.%s IS NULL\nWHERE t.%s IS NULL OR (%s)",
			qualified, ValidFrom, ValidTo, staging, qualified, uk, uk, ValidTo, uk, checkCondition,
		),
		fmt.Sprintf("DROP TABLE IF EXISTS %s", staging),
	}
}

func clickhouseCheckRun(qualified, sourceSQL string, cfg ResolvedConfig) []string {
	// For ClickHouse, append-only is used in v1. We do not support row-level UPDATEs.
	return []string{
		fmt.Sprintf(
			"INSERT INTO %s\nSELECT s.*, now() AS %s, CAST(NULL AS Nullable(DateTime)) AS %s\nFROM (\n%s\n) AS s",
			qualified, ValidFrom, ValidTo, sourceSQL,
		),
	}
}
