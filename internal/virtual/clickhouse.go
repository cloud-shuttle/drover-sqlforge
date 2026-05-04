package virtual

import "fmt"

func ClickHouseZeroCopyClone(sourceDB, sourceTable, targetDB, targetTable string) string {
	return fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.%s 
		CLONE AS %s.%s 
		ENGINE = MergeTree 
		ORDER BY tuple();
	`, targetDB, targetTable, sourceDB, sourceTable)
}

func ClickHouseVirtualView(sourceDB, sourceTable, targetDB, targetTable string) string {
	return fmt.Sprintf(`
		CREATE OR REPLACE VIEW %s.%s AS 
		SELECT * FROM %s.%s;
	`, targetDB, targetTable, sourceDB, sourceTable)
}
