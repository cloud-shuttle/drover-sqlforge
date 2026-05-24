package parser

import (
	"regexp"
	"strings"
)

var (
	asAliasRE   = regexp.MustCompile(`(?i)\s+AS\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*$`)
	qualColRE   = regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z_][a-zA-Z0-9_]*)\b`)
	fromJoinRE  = regexp.MustCompile(`(?i)(?:FROM|JOIN)\s+([a-zA-Z_][a-zA-Z0-9_]*)(?:\s+(?:AS\s+)?([a-zA-Z_][a-zA-Z0-9_]*))?`)
	sqlKeywords = map[string]bool{
		"SELECT": true, "FROM": true, "JOIN": true, "LEFT": true, "RIGHT": true, "INNER": true,
		"OUTER": true, "ON": true, "WHERE": true, "GROUP": true, "BY": true, "ORDER": true,
		"HAVING": true, "LIMIT": true, "AS": true, "AND": true, "OR": true, "NOT": true,
		"CASE": true, "WHEN": true, "THEN": true, "ELSE": true, "END": true, "NULL": true,
		"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true, "DISTINCT": true,
		"CAST": true, "INTERVAL": true, "NOW": true, "TODAY": true,
	}
)

// ExtractColumnLineage derives output-column → upstream-column mappings from SELECT SQL.
// It attempts to use the embedded WASM module first. If the module is a stub or fails,
// it falls back to a v1 structural parser (SELECT list + FROM/JOIN aliases).
func (p *Parser) ExtractColumnLineage(sql string) ([]ColumnMapping, error) {
	// Attempt WASM extraction first
	if mappings, err := p.ExtractColumnLineageWASM(sql); err == nil && mappings != nil {
		return mappings, nil
	}

	// Fallback to v1 structural parsing
	selectList, ok := extractSelectList(stripLineComments(sql))
	if !ok {
		return nil, nil
	}

	aliases := extractTableAliases(sql)
	var mappings []ColumnMapping
	for _, item := range splitSelectList(selectList) {
		if item == "" {
			continue
		}
		out := outputColumnName(item)
		sources := collectColumnRefs(item, aliases)
		mappings = append(mappings, ColumnMapping{
			Output:  out,
			Sources: dedupeRefs(sources),
		})
	}
	return mappings, nil
}

func stripLineComments(sql string) string {
	var b strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func extractSelectList(sql string) (string, bool) {
	upper := strings.ToUpper(sql)
	sel := strings.Index(upper, "SELECT")
	if sel < 0 {
		return "", false
	}
	body := sql[sel+len("SELECT"):]
	depth := 0
	for i := 0; i+4 <= len(body); i++ {
		switch body[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && strings.EqualFold(body[i:i+4], "FROM") {
			if i == 0 || !isIdentChar(body[i-1]) {
				if i+4 >= len(body) || !isIdentChar(body[i+4]) {
					return strings.TrimSpace(body[:i]), true
				}
			}
		}
	}
	return "", false
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func splitSelectList(list string) []string {
	var items []string
	var cur strings.Builder
	depth := 0
	for _, r := range list {
		switch r {
		case '(':
			depth++
			cur.WriteRune(r)
		case ')':
			if depth > 0 {
				depth--
			}
			cur.WriteRune(r)
		case ',':
			if depth == 0 {
				items = append(items, strings.TrimSpace(cur.String()))
				cur.Reset()
				continue
			}
			cur.WriteRune(r)
		default:
			cur.WriteRune(r)
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		items = append(items, s)
	}
	return items
}

func extractTableAliases(sql string) map[string]string {
	aliasToRelation := make(map[string]string)
	for _, m := range fromJoinRE.FindAllStringSubmatch(sql, -1) {
		relation := m[1]
		alias := ""
		if len(m) > 2 {
			alias = m[2]
		}
		if alias == "" || sqlKeywords[strings.ToUpper(alias)] {
			aliasToRelation[relation] = relation
			continue
		}
		aliasToRelation[alias] = relation
		aliasToRelation[relation] = relation
	}
	return aliasToRelation
}

func outputColumnName(item string) string {
	if m := asAliasRE.FindStringSubmatch(item); len(m) > 1 {
		return m[1]
	}
	item = strings.TrimSpace(item)
	if dot := strings.LastIndex(item, "."); dot >= 0 && dot+1 < len(item) {
		tail := item[dot+1:]
		if isSimpleIdent(tail) {
			return tail
		}
	}
	if isSimpleIdent(item) {
		return item
	}
	return summarizeExpr(item)
}

func summarizeExpr(expr string) string {
	expr = strings.TrimSpace(expr)
	if len(expr) <= 48 {
		return expr
	}
	return expr[:45] + "..."
}

func isSimpleIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 && r >= '0' && r <= '9' {
			return false
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func collectColumnRefs(item string, aliases map[string]string) []ColumnRef {
	seen := make(map[string]struct{})
	var refs []ColumnRef
	for _, m := range qualColRE.FindAllStringSubmatch(item, -1) {
		relAlias, col := m[1], m[2]
		if sqlKeywords[strings.ToUpper(relAlias)] {
			continue
		}
		relation := relAlias
		if resolved, ok := aliases[relAlias]; ok {
			relation = resolved
		}
		key := relation + "." + col
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		refs = append(refs, ColumnRef{Relation: relation, Column: col})
	}
	return refs
}

func dedupeRefs(refs []ColumnRef) []ColumnRef {
	if len(refs) == 0 {
		return refs
	}
	seen := make(map[string]struct{}, len(refs))
	out := make([]ColumnRef, 0, len(refs))
	for _, r := range refs {
		key := r.Relation + "." + r.Column
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, r)
	}
	return out
}
