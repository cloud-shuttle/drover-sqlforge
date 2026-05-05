package parser

import (
	"strings"
	"unicode"
)

// ReplaceDependencies takes a raw SQL string and safely replaces table dependencies
// (e.g. "stg_users" -> "sqlforge__dev.stg_users") by tokenizing the SQL and ignoring
// string literals, comments, and column aliases.
func ReplaceDependencies(sql string, deps map[string]string) string {
	if len(deps) == 0 {
		return sql
	}

	var result strings.Builder
	var currentToken strings.Builder

	inString := false
	inDoubleQuote := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	lastKeyword := ""

	runes := []rune(sql)
	length := len(runes)

	flushToken := func() {
		if currentToken.Len() == 0 {
			return
		}
		tokenStr := currentToken.String()
		upperToken := strings.ToUpper(tokenStr)

		// Is it a known SQL keyword?
		isKeyword := false
		switch upperToken {
		case "AS", "WITH", "FROM", "JOIN", "ON", "WHERE", "GROUP", "ORDER", "HAVING", "LIMIT", "SELECT":
			isKeyword = true
			lastKeyword = upperToken
		}

		if !isKeyword {
			// Check if we should replace
			if replacement, exists := deps[strings.ToLower(tokenStr)]; exists {
				if lastKeyword == "AS" || lastKeyword == "WITH" {
					// It's an alias or CTE definition, do not replace
					result.WriteString(tokenStr)
				} else {
					result.WriteString(replacement)
				}
			} else {
				result.WriteString(tokenStr)
			}
		} else {
			result.WriteString(tokenStr)
		}
		currentToken.Reset()
	}

	for i := 0; i < length; i++ {
		r := runes[i]

		if inLineComment {
			if r == '\n' {
				inLineComment = false
			}
			result.WriteRune(r)
			continue
		}

		if inBlockComment {
			if r == '*' && i+1 < length && runes[i+1] == '/' {
				inBlockComment = false
				result.WriteRune(r)
				result.WriteRune('/')
				i++
			} else {
				result.WriteRune(r)
			}
			continue
		}

		if inString {
			result.WriteRune(r)
			if r == '\'' {
				if i+1 < length && runes[i+1] == '\'' {
					result.WriteRune('\'')
					i++
				} else {
					inString = false
				}
			}
			continue
		}

		if inDoubleQuote {
			currentToken.WriteRune(r)
			if r == '"' {
				inDoubleQuote = false
				flushToken()
			}
			continue
		}

		if inBacktick {
			currentToken.WriteRune(r)
			if r == '`' {
				inBacktick = false
				flushToken()
			}
			continue
		}

		if r == '-' && i+1 < length && runes[i+1] == '-' {
			flushToken()
			inLineComment = true
			result.WriteString("--")
			i++
			continue
		}
		if r == '/' && i+1 < length && runes[i+1] == '*' {
			flushToken()
			inBlockComment = true
			result.WriteString("/*")
			i++
			continue
		}

		if r == '\'' {
			flushToken()
			inString = true
			result.WriteRune(r)
			continue
		}
		if r == '"' {
			flushToken()
			inDoubleQuote = true
			currentToken.WriteRune(r)
			continue
		}
		if r == '`' {
			flushToken()
			inBacktick = true
			currentToken.WriteRune(r)
			continue
		}

		if unicode.IsSpace(r) || r == ',' || r == '(' || r == ')' || r == ';' || r == '=' || r == '.' {
			flushToken()
			result.WriteRune(r)
		} else {
			currentToken.WriteRune(r)
		}
	}

	flushToken()
	return result.String()
}
