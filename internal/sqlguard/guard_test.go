package sqlguard

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestValidateSelect_Allows(t *testing.T) {
	ok := []string{
		"SELECT 1",
		"select id, name from sqlforge__dev.customers",
		"SELECT 1;",
		"SELECT 1;  \n",
		"(SELECT 1)",
		"WITH a AS (SELECT 1 AS x) SELECT x FROM a",
		"WITH a AS (SELECT 1), b AS (SELECT * FROM a) SELECT * FROM b",
		"SELECT 'a;b' AS s",
		"SELECT 'it''s; fine' AS s",
		"SELECT \"delete\", \"load\" FROM t",
		"SELECT updated_at, deleted_flag, created_by FROM t",
		"SELECT 1 -- trailing comment; DROP TABLE x\n",
		"SELECT /* ; DROP TABLE x */ 1",
		"-- @materialized: table\nSELECT id FROM src",
		"SELECT CAST(x AS INT), EXTRACT(year FROM ts), INTERVAL '1 day' FROM t",
		"SELECT a # b FROM t",
		"SELECT count(*) FILTER (WHERE status = 'paid') FROM orders GROUP BY 1",
	}
	for _, sql := range ok {
		if err := ValidateSelect(sql); err != nil {
			t.Errorf("ValidateSelect(%q) = %v, want nil", sql, err)
		}
	}
}

func TestValidateSelect_Rejects(t *testing.T) {
	cases := map[string]string{
		"":                                   "empty",
		"   ;  ":                             "empty",
		"SELECT 1; DROP SCHEMA prod CASCADE": "multiple_statements",
		"SELECT 1;;":                         "multiple_statements",
		"SELECT 1; SELECT 2":                 "multiple_statements",
		"DROP TABLE x":                       "not_select",
		"INSERT INTO t VALUES (1)":           "not_select",
		"UPDATE t SET a = 1":                 "not_select",
		"VALUES (1)":                         "not_select",
		"SELECT * INTO new_t FROM t":         "forbidden_keyword",
		"WITH d AS (DELETE FROM t RETURNING *) SELECT * FROM d":   "forbidden_keyword",
		"SELECT * FROM t INTO OUTFILE '/tmp/x'":                   "forbidden_keyword",
		"SELECT * FROM read_csv('/etc/passwd')":                   "forbidden_function",
		"SELECT * FROM read_text ('/etc/shadow')":                 "forbidden_function",
		"SELECT pg_read_file('/etc/passwd')":                      "forbidden_function",
		"SELECT * FROM url('http://169.254.169.254/latest', CSV)": "forbidden_function",
		"SELECT * FROM s3('https://bucket/x')":                    "forbidden_function",
		"SELECT * FROM file('/etc/passwd', 'LineAsString')":       "forbidden_function",
		"SELECT pg_sleep(100)":                                    "forbidden_function",
		"SELECT SYSTEM$WHITELIST()":                               "forbidden_function",
		"SELECT 'unterminated":                                    "unterminated",
		"SELECT /* unterminated":                                  "unterminated",
		"SELECT \"unterminated":                                   "unterminated",
		"SELECT $a$ unterminated":                                 "unterminated",
		// Dialect-divergent lexing tricks.
		"SELECT 'a\\'; DROP TABLE x; --'":       "multiple_statements",
		"SELECT 1 # x\n; DROP TABLE x":          "multiple_statements",
		"SELECT 1 # ; DROP TABLE x":             "multiple_statements",
		"SELECT /* /* */ ;DROP TABLE x -- */ 1": "multiple_statements",
		"SELECT $a$; DROP TABLE x; $a$":         "multiple_statements",
		"SELECT `a`; DROP TABLE x; `b`":         "multiple_statements",
		"SELECT \"a\\\"; DROP TABLE x; \"":      "unterminated",
		// Conservative by design: backticks and dollar quotes are not universal,
		// so their contents must also be safe as bare code.
		"SELECT `update` FROM t":                "forbidden_keyword",
		"SELECT $$a;b$$":                        "multiple_statements",
		"SELECT * FROM system.numbers LIMIT 10": "",
		"SELECT 1\x00; DROP TABLE x":            "nul_byte",
		"SELECT 1 /*!50000 ; DROP TABLE x */":   "", // hidden inside a comment in every reading: allowed
	}
	for sql, rule := range cases {
		err := ValidateSelect(sql)
		if rule == "" {
			if err != nil {
				t.Errorf("ValidateSelect(%q) = %v, want nil", sql, err)
			}
			continue
		}
		var ge *Error
		if !errors.As(err, &ge) {
			t.Errorf("ValidateSelect(%q) = %v, want rule %s", sql, err, rule)
			continue
		}
		if ge.Rule != rule {
			// Several rules can apply; any rejection is safe, but keep the
			// expected rule precise so regressions are visible.
			t.Errorf("ValidateSelect(%q) rule = %s (%s), want %s", sql, ge.Rule, ge.Detail, rule)
		}
	}
}

func TestValidateSelect_TooLarge(t *testing.T) {
	sql := "SELECT '" + strings.Repeat("a", MaxSQLBytes) + "'"
	var ge *Error
	if err := ValidateSelect(sql); !errors.As(err, &ge) || ge.Rule != "too_large" {
		t.Fatalf("err = %v, want too_large", err)
	}
}

// Every model shipped in examples/ must pass, so the guard does not block
// realistic agent edits.
func TestValidateSelect_ExampleModels(t *testing.T) {
	root := filepath.Join("..", "..", "examples")
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".sql") || !strings.Contains(path, string(filepath.Separator)+"models"+string(filepath.Separator)) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		count++
		if err := ValidateSelect(string(b)); err != nil {
			t.Errorf("%s: %v", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no example models found")
	}
}

// Appending "; <anything>" to a valid SELECT must always be rejected.
func TestValidateSelect_PropertySecondStatement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tail := rapid.StringMatching(`[A-Za-z ]{1,20}`).Draw(t, "tail")
		sql := "SELECT id FROM t; " + tail
		if strings.TrimSpace(tail) == "" {
			return
		}
		if err := ValidateSelect(sql); err == nil {
			t.Fatalf("accepted %q", sql)
		}
	})
}

// The guard must never panic on arbitrary input.
func TestValidateSelect_PropertyNoPanic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		_ = ValidateSelect(rapid.String().Draw(t, "sql"))
	})
}

func FuzzValidateSelect(f *testing.F) {
	for _, s := range []string{"SELECT 1", "SELECT 'a\\'; DROP x", "/* /* */ ;", "$a$;$a$"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, sql string) {
		err := ValidateSelect(sql)
		if err == nil {
			// Anything accepted must contain no bare semicolon other than a
			// trailing one under the most literal reading.
			code, _ := stripLiterals(sql, lexMode{})
			c := strings.TrimSpace(code)
			c = strings.TrimSuffix(c, ";")
			if strings.Contains(c, ";") {
				t.Fatalf("accepted multi-statement %q", sql)
			}
		}
	})
}
