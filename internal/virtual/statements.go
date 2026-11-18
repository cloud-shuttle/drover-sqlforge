package virtual

import (
	"context"
	"database/sql"
	"strings"
)

// SplitStatements divides sql on semicolons that sit outside quotes and
// comments. Empty pieces are dropped. The semicolon itself is not included.
func SplitStatements(sql string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(sql); {
		next := consumeToken(sql, i)
		if next > i {
			i = next
			continue
		}
		if sql[i] == ';' {
			appendStatement(&parts, sql[start:i])
			i++
			start = i
			continue
		}
		i++
	}
	appendStatement(&parts, sql[start:])
	return parts
}

// ExecStatements runs each top-level statement from query. It stops at the
// first error and does not run the rest.
func ExecStatements(ctx context.Context, query string, exec func(context.Context, string) error) error {
	for _, stmt := range SplitStatements(query) {
		if err := exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// ExecSQL runs query as one transaction, one top-level statement at a time.
// A failure rolls the transaction back, so a delete-then-insert merge cannot
// leave the table deleted when the insert fails.
func ExecSQL(ctx context.Context, db *sql.DB, query string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := ExecStatements(ctx, query, func(ctx context.Context, stmt string) error {
		_, err := tx.ExecContext(ctx, stmt)
		return err
	}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func appendStatement(parts *[]string, stmt string) {
	stmt = strings.TrimSpace(stmt)
	if stmt == "" {
		return
	}
	*parts = append(*parts, stmt)
}

// consumeToken returns the index after a quote or comment starting at i,
// or i when sql[i] is ordinary text.
func consumeToken(sql string, i int) int {
	if i >= len(sql) {
		return i
	}
	if hasPrefix(sql, i, "--") {
		return skipLineComment(sql, i)
	}
	if hasPrefix(sql, i, "/*") {
		return skipBlockComment(sql, i)
	}
	switch sql[i] {
	case '\'', '"':
		return skipQuoted(sql, i, sql[i])
	case '$':
		return skipDollar(sql, i)
	default:
		return i
	}
}

func hasPrefix(sql string, i int, prefix string) bool {
	return i+len(prefix) <= len(sql) && sql[i:i+len(prefix)] == prefix
}

func skipLineComment(sql string, i int) int {
	i += 2
	for i < len(sql) && sql[i] != '\n' {
		i++
	}
	return i
}

func skipBlockComment(sql string, i int) int {
	i += 2
	for i+1 < len(sql) {
		if sql[i] == '*' && sql[i+1] == '/' {
			return i + 2
		}
		i++
	}
	return len(sql)
}

func skipQuoted(sql string, i int, quote byte) int {
	i++
	for i < len(sql) {
		if sql[i] == quote {
			if i+1 < len(sql) && sql[i+1] == quote {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return len(sql)
}

func skipDollar(sql string, i int) int {
	if i >= len(sql) || sql[i] != '$' {
		return i
	}
	j := i + 1
	for j < len(sql) && isTagChar(sql[j]) {
		j++
	}
	if j >= len(sql) || sql[j] != '$' {
		return i
	}
	tag := sql[i : j+1]
	k := strings.Index(sql[j+1:], tag)
	if k < 0 {
		return len(sql)
	}
	return j + 1 + k + len(tag)
}

func isTagChar(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_'
}
