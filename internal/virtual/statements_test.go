package virtual

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestSplitStatements(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []string
	}{
		{name: "empty", sql: "  \n\t ", want: nil},
		{name: "one", sql: "SELECT 1", want: []string{"SELECT 1"}},
		{name: "trailing", sql: "SELECT 1;", want: []string{"SELECT 1"}},
		{name: "two", sql: "DROP TABLE t;\nCREATE TABLE t AS\nSELECT 1", want: []string{"DROP TABLE t", "CREATE TABLE t AS\nSELECT 1"}},
		{name: "blank between", sql: "SELECT 1;  ; SELECT 2;", want: []string{"SELECT 1", "SELECT 2"}},
		{name: "quoted", sql: "SELECT 'keep;me'; SELECT 2", want: []string{"SELECT 'keep;me'", "SELECT 2"}},
		{name: "doubled quote", sql: "SELECT 'it'';s;fine'; SELECT 2", want: []string{"SELECT 'it'';s;fine'", "SELECT 2"}},
		{name: "ident", sql: "SELECT \"a;b\"; SELECT 2", want: []string{"SELECT \"a;b\"", "SELECT 2"}},
		{name: "line comment", sql: "SELECT 1; -- hide; this\nSELECT 2", want: []string{"SELECT 1", "-- hide; this\nSELECT 2"}},
		{name: "block comment", sql: "SELECT 1 /* ; */ ; SELECT 2", want: []string{"SELECT 1 /* ; */", "SELECT 2"}},
		{name: "dollar", sql: "SELECT $$ a;b $$; SELECT 2", want: []string{"SELECT $$ a;b $$", "SELECT 2"}},
		{name: "tagged dollar", sql: "SELECT $body$ a;b $body$; SELECT 2", want: []string{"SELECT $body$ a;b $body$", "SELECT 2"}},
		{name: "unterminated dollar", sql: "SELECT $$ a;b", want: []string{"SELECT $$ a;b"}},
		{name: "not dollar", sql: "SELECT $1; SELECT 2", want: []string{"SELECT $1", "SELECT 2"}},
		{name: "unterminated quote", sql: "SELECT 'a;b", want: []string{"SELECT 'a;b"}},
		{name: "unterminated block", sql: "SELECT 1 /* ;", want: []string{"SELECT 1 /* ;"}},
		{name: "comment at end", sql: "SELECT 1; -- tail", want: []string{"SELECT 1", "-- tail"}},
		{
			name: "ddl with quoted semicolon",
			sql:  "DROP TABLE IF EXISTS s.t CASCADE;\nCREATE TABLE s.t AS\nSELECT 'keep;me' AS note;",
			want: []string{
				"DROP TABLE IF EXISTS s.t CASCADE",
				"CREATE TABLE s.t AS\nSELECT 'keep;me' AS note",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitStatements(tt.sql)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

func TestExecStatementsStopsOnError(t *testing.T) {
	var got []string
	err := ExecStatements(context.Background(), "SELECT 1; SELECT 2", func(_ context.Context, stmt string) error {
		got = append(got, stmt)
		if len(got) == 1 {
			return errors.New("boom")
		}
		return nil
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"SELECT 1"}) {
		t.Fatalf("ran %v", got)
	}
}

func TestExecStatementsEmpty(t *testing.T) {
	called := false
	if err := ExecStatements(context.Background(), " ; ", func(context.Context, string) error {
		called = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("empty script executed")
	}
}

func TestPostgresStubExecDoesNotSplitRun(t *testing.T) {
	r, err := NewPostgresRunner("stub")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Exec(context.Background(), "SELECT 1; DROP TABLE x"); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}
