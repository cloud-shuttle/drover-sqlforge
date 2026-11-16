// Package sqlident validates identifiers and quotes literals before they are
// interpolated into generated SQL. SQLForge builds DDL and data-quality
// queries with string formatting, so every name that reaches SQL text
// (environment, schema, database, table, column) must be a bare identifier,
// and every value must be an escaped literal.
package sqlident

import (
	"fmt"
	"regexp"
	"strings"
)

// MaxIdentifierLen is the PostgreSQL limit (63 bytes), the tightest of the
// supported warehouses.
const MaxIdentifierLen = 63

// envSchemaPrefix is prepended to an environment name to form its schema.
const envSchemaPrefix = "sqlforge__"

var (
	identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	envRe   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
)

// ValidateIdentifier returns an error unless name is a bare SQL identifier:
// ASCII letters, digits and underscores, not starting with a digit, at most
// MaxIdentifierLen bytes. kind names the thing in the error ("model", ...).
func ValidateIdentifier(kind, name string) error {
	if name == "" {
		return fmt.Errorf("invalid %s name: empty", kind)
	}
	if len(name) > MaxIdentifierLen {
		return fmt.Errorf("invalid %s name %q: longer than %d characters", kind, name, MaxIdentifierLen)
	}
	if !identRe.MatchString(name) {
		return fmt.Errorf("invalid %s name %q: use letters, digits and underscores only, not starting with a digit", kind, name)
	}
	return nil
}

// ValidateQualified validates a dot-separated name (database.schema.table);
// each part must be a bare identifier.
func ValidateQualified(kind, name string) error {
	if name == "" {
		return fmt.Errorf("invalid %s name: empty", kind)
	}
	for _, part := range strings.Split(name, ".") {
		if err := ValidateIdentifier(kind, part); err != nil {
			return fmt.Errorf("invalid %s name %q: %w", kind, name, err)
		}
	}
	return nil
}

// ValidateEnvName validates an environment name. It must start with a letter,
// and the derived schema ("sqlforge__" + name) must fit MaxIdentifierLen.
func ValidateEnvName(name string) error {
	maxLen := MaxIdentifierLen - len(envSchemaPrefix)
	if name == "" {
		return fmt.Errorf("invalid environment name: empty")
	}
	if len(name) > maxLen {
		return fmt.Errorf("invalid environment name %q: longer than %d characters", name, maxLen)
	}
	if !envRe.MatchString(name) {
		return fmt.Errorf("invalid environment name %q: start with a letter and use letters, digits and underscores only", name)
	}
	return nil
}

// ValidateColumnList validates a comma-separated list of column names
// (e.g. unique_key = "id, tenant_id"). Empty entries are ignored; at least one
// column is required.
func ValidateColumnList(kind, list string) error {
	n := 0
	for _, c := range strings.Split(list, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if err := ValidateIdentifier(kind, c); err != nil {
			return err
		}
		n++
	}
	if n == 0 {
		return fmt.Errorf("invalid %s: no columns", kind)
	}
	return nil
}

// backslashEscapes reports whether a dialect treats backslash as an escape
// character inside single-quoted string literals.
func backslashEscapes(dialect string) bool {
	switch strings.ToLower(dialect) {
	case "clickhouse", "mysql", "doris", "velodb", "databricks", "snowflake":
		return true
	default: // postgres, duckdb, ansi
		return false
	}
}

// QuoteLiteral returns v as a single-quoted SQL string literal for dialect.
// Single quotes are doubled everywhere; backslashes are doubled for dialects
// that treat them as escapes. NUL bytes are dropped.
func QuoteLiteral(dialect, v string) string {
	v = strings.ReplaceAll(v, "\x00", "")
	if backslashEscapes(dialect) {
		v = strings.ReplaceAll(v, `\`, `\\`)
	}
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}
