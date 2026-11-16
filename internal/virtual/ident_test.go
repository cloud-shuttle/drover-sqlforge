package virtual

import (
	"context"
	"strings"
	"testing"

	"github.com/cloud-shuttle/drover-sqlforge/internal/virtual/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIncrementalBuilders_RejectUnsafeIdentifiers(t *testing.T) {
	bad := []struct {
		schema, table string
		cfg           map[string]string
	}{
		{"s; DROP SCHEMA x", "t", map[string]string{}},
		{"s", "t; DROP TABLE x", map[string]string{}},
		{"s", "t", map[string]string{"unique_key": "id) OR (1=1"}},
		{"s", "t", map[string]string{"unique_key": "id", "updated_at": "ts; --"}},
	}
	for _, d := range []string{"postgres", "duckdb", "clickhouse", "snowflake"} {
		for _, c := range bad {
			if _, err := BuildIncrementalMergeDDL(d, c.schema, c.table, "SELECT 1", c.cfg); err == nil {
				t.Errorf("merge %s %v: want error", d, c)
			}
			if _, err := BuildIncrementalInitialDDL(d, c.schema, c.table, "SELECT 1", c.cfg); err == nil {
				t.Errorf("initial %s %v: want error", d, c)
			}
		}
		if _, err := BuildIncrementalMergeDDL(d, "db.s", "t", "SELECT 1", map[string]string{"unique_key": "id, tenant_id"}); err != nil {
			t.Errorf("%s: valid merge rejected: %v", d, err)
		}
	}
}

func TestClickHouseStreamingDDL_EscapesSettings(t *testing.T) {
	r := &ClickHouseRunner{stub: true}
	ddl := r.CreateStreamingTableDDL("s", "t", map[string]string{
		"_materialization_type": "kafka",
		"kafka_broker_list":     "b:9092', kafka_format = 'X",
		"kafka_topic_list":      `t\`,
		"kafka_group_name":      "g",
	})
	if !strings.Contains(ddl, `kafka_broker_list = 'b:9092'', kafka_format = ''X'`) {
		t.Errorf("broker not escaped:\n%s", ddl)
	}
	if !strings.Contains(ddl, `kafka_topic_list = 't\\'`) {
		t.Errorf("backslash not escaped:\n%s", ddl)
	}
	ddl = r.CreateStreamingTableDDL("s", "t", map[string]string{
		"_materialization_type": "nats",
		"nats_url":              "nats://x' SETTINGS y='z",
	})
	if !strings.Contains(ddl, `nats_url = 'nats://x'' SETTINGS y=''z'`) {
		t.Errorf("nats url not escaped:\n%s", ddl)
	}
}

func TestPluginGRPCServer_RejectsUnsafeIdentifiers(t *testing.T) {
	s := &GRPCServer{Impl: &ClickHouseRunner{stub: true}}
	ctx := context.Background()
	check := func(name string, err error) {
		t.Helper()
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: code = %v, want InvalidArgument (err=%v)", name, status.Code(err), err)
		}
	}
	_, err := s.CreateSchemaDDL(ctx, &proto.CreateSchemaRequest{Schema: "x; DROP SCHEMA y"})
	check("schema", err)
	_, err = s.CreateTableDDL(ctx, &proto.CreateTableRequest{Schema: "s", Table: "t; DROP", SelectSql: "SELECT 1"})
	check("table", err)
	_, err = s.CreateViewDDL(ctx, &proto.CreateViewRequest{Schema: "s--", Table: "t", SelectSql: "SELECT 1"})
	check("view", err)
	_, err = s.CreateMaterializedViewDDL(ctx, &proto.CreateMaterializedViewRequest{Schema: "s", Table: "t t"})
	check("mview", err)
	_, err = s.CreateStreamingTableDDL(ctx, &proto.CreateStreamingTableRequest{Schema: "s", Table: "t'"})
	check("stream", err)
	_, err = s.CreateIncrementalMergeDDL(ctx, &proto.CreateIncrementalMergeRequest{Schema: "s", Table: "1t"})
	check("merge", err)

	resp, err := s.CreateTableDDL(ctx, &proto.CreateTableRequest{Schema: "db.s", Table: "orders", SelectSql: "SELECT 1"})
	if err != nil || !strings.Contains(resp.Ddl, "db.s.orders") {
		t.Fatalf("valid request: resp=%v err=%v", resp, err)
	}
}
