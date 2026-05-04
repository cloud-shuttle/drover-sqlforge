package virtual

import (
	"fmt"
	"strings"
)

func NewRunner(dialect string, dsn string) (Runner, error) {
	dialect = strings.ToLower(strings.TrimSpace(dialect))

	switch dialect {
	case "clickhouse", "":
		return NewClickHouseRunner(dsn)
	case "duckdb":
		return NewDuckDBRunner(dsn)
	case "postgres":
		return NewPostgresRunner(dsn)
	case "snowflake":
		return NewSnowflakeRunner(dsn)
	case "databricks":
		return NewDatabricksRunner(dsn)
	case "doris":
		return NewDorisRunner(dsn)
	case "velodb":
		return NewVeloDBRunner(dsn)
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}
}
