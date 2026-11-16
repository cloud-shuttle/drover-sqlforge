// Package sqlguard checks that SQL supplied by an untrusted caller (an MCP
// agent proposing a model change) is a single read-only SELECT before it is
// wrapped in CREATE ... AS and executed against a warehouse.
//
// The check is deliberately conservative: it is a lexical filter, not a full
// parser, and it rejects anything it cannot prove is a single SELECT/WITH
// statement. Models that data engineers commit to Git are not run through it.
package sqlguard

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// MaxSQLBytes bounds proposed model SQL.
const MaxSQLBytes = 256 << 10 // 256 KiB

// Error describes why SQL was rejected. Rule is a short stable identifier.
type Error struct {
	Rule   string
	Detail string
}

func (e *Error) Error() string {
	return fmt.Sprintf("proposed SQL rejected (%s): %s", e.Rule, e.Detail)
}

// forbiddenKeywords may not appear as bare words anywhere outside string
// literals, quoted identifiers and comments. Quote an identifier with ANSI
// double quotes ("load") if a column genuinely uses one of these names;
// backticks and $tag$ strings are not treated as quoting because not every
// dialect honours them. (ClickHouse SYSTEM statements are already excluded by
// the single-SELECT rule, so system.numbers and friends stay usable.)
var forbiddenKeywords = []string{
	"INSERT", "UPDATE", "DELETE", "MERGE", "UPSERT", "DROP", "ALTER", "CREATE",
	"TRUNCATE", "RENAME", "GRANT", "REVOKE", "COPY", "ATTACH", "DETACH", "CALL",
	"EXEC", "EXECUTE", "DO", "SET", "RESET", "PRAGMA", "INSTALL", "LOAD",
	"VACUUM", "ANALYZE", "OPTIMIZE", "KILL", "BEGIN", "COMMIT",
	"ROLLBACK", "SAVEPOINT", "LOCK", "UNLOCK", "INTO", "OUTFILE", "DUMPFILE",
	"USE", "SHOW", "DESCRIBE", "EXPLAIN", "PREPARE", "DEALLOCATE", "LISTEN",
	"NOTIFY", "REFRESH", "EXPORT", "IMPORT", "UNDROP", "PUT", "GET", "REMOVE",
}

// forbiddenFunctions read or write files, reach the network, or stall the
// server. Matched case-insensitively when followed by "(".
var forbiddenFunctions = []string{
	// DuckDB
	"read_csv", "read_csv_auto", "read_parquet", "parquet_scan", "read_json",
	"read_json_auto", "read_json_objects", "read_ndjson", "read_ndjson_auto",
	"read_text", "read_blob", "glob", "sniff_csv", "postgres_scan",
	"postgres_query", "mysql_scan", "mysql_query", "sqlite_scan", "iceberg_scan",
	"delta_scan", "query_table", "query",
	// PostgreSQL
	"pg_read_file", "pg_read_binary_file", "pg_ls_dir", "pg_stat_file",
	"lo_import", "lo_export", "dblink", "dblink_exec", "dblink_connect",
	"pg_sleep", "pg_terminate_backend", "pg_cancel_backend", "set_config",
	"current_setting", "query_to_xml",
	// ClickHouse
	"file", "url", "urlcluster", "s3", "s3cluster", "gcs", "azureblobstorage",
	"hdfs", "remote", "remotesecure", "cluster", "clusterallreplicas", "mysql",
	"postgresql", "mongodb", "redis", "jdbc", "odbc", "executable", "input",
	"sleep", "sleepeachrow",
	// Snowflake / Databricks / MySQL-wire
	"system$", "load_file", "benchmark",
}

var (
	wordRe        = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_$]*`)
	keywordSet    = toSet(forbiddenKeywords)
	functionSet   = toSet(forbiddenFunctions)
	funcCallRe    = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_$]*)\s*\(`)
	dollarTagRe   = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)?\$`)
	errUnterminal = errors.New("unterminated literal, identifier or comment")
)

func toSet(words []string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[strings.ToUpper(w)] = true
	}
	return m
}

// ValidateSelect returns nil if sql is exactly one read-only SELECT (or WITH
// ... SELECT) statement, optionally followed by a single trailing semicolon.
func ValidateSelect(sql string) error {
	if strings.TrimSpace(sql) == "" {
		return &Error{"empty", "SQL is empty"}
	}
	if len(sql) > MaxSQLBytes {
		return &Error{"too_large", fmt.Sprintf("SQL exceeds %d bytes", MaxSQLBytes)}
	}
	if strings.ContainsRune(sql, 0) {
		return &Error{"nul_byte", "SQL contains a NUL byte"}
	}
	// Dialects disagree on lexing: whether backslash escapes a quote ('a\' is
	// a complete literal in PostgreSQL but not in MySQL or ClickHouse), whether
	// # starts a comment (MySQL) or is an operator (PostgreSQL), whether block
	// comments nest, and whether $tag$ and backticks quote. SQL must pass under
	// every combination so no dialect can see a second statement that another
	// reading hides inside a literal or comment.
	for bits := 0; bits < 1<<5; bits++ {
		m := lexMode{
			backslashEscapes: bits&1 != 0,
			hashComments:     bits&2 != 0,
			nestedComments:   bits&4 != 0,
			dollarQuotes:     bits&8 != 0,
			backtickQuotes:   bits&16 != 0,
		}
		code, err := stripLiterals(sql, m)
		if err != nil {
			return &Error{"unterminated", err.Error()}
		}
		if err := checkCode(code); err != nil {
			return err
		}
	}
	return nil
}

// lexMode selects one dialect reading of quoting and comment rules.
type lexMode struct {
	backslashEscapes bool
	hashComments     bool
	nestedComments   bool
	dollarQuotes     bool
	backtickQuotes   bool
}

func checkCode(code string) error {
	trimmed := strings.TrimSpace(code)
	trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
	if strings.Contains(trimmed, ";") {
		return &Error{"multiple_statements", "only a single statement is allowed"}
	}
	if trimmed == "" {
		return &Error{"empty", "SQL has no statement"}
	}

	first := wordRe.FindString(strings.TrimLeft(trimmed, "( \t\r\n"))
	switch strings.ToUpper(first) {
	case "SELECT", "WITH":
	default:
		return &Error{"not_select", fmt.Sprintf("statement must start with SELECT or WITH, got %q", first)}
	}

	for _, w := range wordRe.FindAllString(trimmed, -1) {
		if keywordSet[strings.ToUpper(w)] {
			return &Error{"forbidden_keyword", fmt.Sprintf("%s is not allowed in a model", strings.ToUpper(w))}
		}
	}
	for _, m := range funcCallRe.FindAllStringSubmatch(trimmed, -1) {
		name := strings.ToUpper(m[1])
		if functionSet[name] || strings.HasPrefix(name, "SYSTEM$") {
			return &Error{"forbidden_function", fmt.Sprintf("function %s() is not allowed in a model", strings.ToLower(m[1]))}
		}
	}
	return nil
}

// stripLiterals replaces string literals, quoted identifiers and comments with
// spaces so that only code remains. Quoted identifiers become a placeholder
// word so "SELECT \"delete\" FROM t" stays a valid SELECT.
func stripLiterals(sql string, m lexMode) (string, error) {
	var b strings.Builder
	b.Grow(len(sql))
	i := 0
	n := len(sql)
	for i < n {
		c := sql[i]
		switch {
		case c == '-' && i+1 < n && sql[i+1] == '-':
			j := strings.IndexByte(sql[i:], '\n')
			if j < 0 {
				return b.String(), nil
			}
			b.WriteByte(' ')
			i += j
		case c == '#' && m.hashComments:
			// MySQL-wire dialects (Doris, VeloDB) treat # as a line comment.
			j := strings.IndexByte(sql[i:], '\n')
			if j < 0 {
				return b.String(), nil
			}
			b.WriteByte(' ')
			i += j
		case c == '/' && i+1 < n && sql[i+1] == '*':
			// PostgreSQL nests block comments; most other dialects end at the
			// first */.
			depth := 1
			j := i + 2
			for j < n && depth > 0 {
				if m.nestedComments && sql[j] == '/' && j+1 < n && sql[j+1] == '*' {
					depth++
					j += 2
				} else if sql[j] == '*' && j+1 < n && sql[j+1] == '/' {
					depth--
					j += 2
				} else {
					j++
				}
			}
			if depth > 0 {
				return "", errUnterminal
			}
			b.WriteByte(' ')
			i = j
		case c == '\'':
			j, err := skipQuoted(sql, i, '\'', m.backslashEscapes)
			if err != nil {
				return "", err
			}
			b.WriteString(" '' ")
			i = j
		case c == '"' || (c == '`' && m.backtickQuotes):
			j, err := skipQuoted(sql, i, c, m.backslashEscapes)
			if err != nil {
				return "", err
			}
			b.WriteString(" quoted_ident ")
			i = j
		case c == '$' && m.dollarQuotes:
			if tag := dollarTagRe.FindString(sql[i:]); tag != "" {
				end := strings.Index(sql[i+len(tag):], tag)
				if end < 0 {
					return "", errUnterminal
				}
				b.WriteString(" '' ")
				i += len(tag) + end + len(tag)
			} else {
				b.WriteByte(c)
				i++
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), nil
}

// skipQuoted returns the index just past the closing quote that matches the
// opening quote at start. A doubled quote is an escaped quote.
func skipQuoted(sql string, start int, q byte, backslashEscapes bool) (int, error) {
	n := len(sql)
	j := start + 1
	for j < n {
		switch {
		case backslashEscapes && sql[j] == '\\':
			j += 2
		case sql[j] == q:
			if j+1 < n && sql[j+1] == q {
				j += 2
				continue
			}
			return j + 1, nil
		default:
			j++
		}
	}
	return 0, errUnterminal
}
