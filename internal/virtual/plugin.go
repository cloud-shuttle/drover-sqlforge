package virtual

import (
	"context"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/drover-org/drover-sqlforge/internal/virtual/proto"
)

// Handshake is a common handshake that is shared by plugin and host.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "SQLFORGE_PLUGIN",
	MagicCookieValue: "sqlforge_virtual_runner",
}

// RunnerGRPCPlugin is the implementation of plugin.GRPCPlugin so we can serve/consume this.
type RunnerGRPCPlugin struct {
	plugin.Plugin
	Impl Runner
}

func (p *RunnerGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterRunnerPluginServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *RunnerGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: proto.NewRunnerPluginClient(c)}, nil
}

// GRPCClient is an implementation of Runner that talks over RPC.
type GRPCClient struct {
	client proto.RunnerPluginClient
}

func (m *GRPCClient) Exec(ctx context.Context, sql string) error {
	_, err := m.client.Exec(ctx, &proto.ExecRequest{Sql: sql})
	return err
}

func (m *GRPCClient) CreateSchemaDDL(schema string) string {
	resp, err := m.client.CreateSchemaDDL(context.Background(), &proto.CreateSchemaRequest{Schema: schema})
	if err != nil {
		return ""
	}
	return resp.Ddl
}

func (m *GRPCClient) CreateTableDDL(schema, table, selectSQL string) string {
	resp, err := m.client.CreateTableDDL(context.Background(), &proto.CreateTableRequest{Schema: schema, Table: table, SelectSql: selectSQL})
	if err != nil {
		return ""
	}
	return resp.Ddl
}

func (m *GRPCClient) CreateViewDDL(schema, table, selectSQL string) string {
	resp, err := m.client.CreateViewDDL(context.Background(), &proto.CreateViewRequest{Schema: schema, Table: table, SelectSql: selectSQL})
	if err != nil {
		return ""
	}
	return resp.Ddl
}

func (m *GRPCClient) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	resp, err := m.client.CreateMaterializedViewDDL(context.Background(), &proto.CreateMaterializedViewRequest{Schema: schema, Table: table, SelectSql: selectSQL})
	if err != nil {
		return ""
	}
	return resp.Ddl
}

func (m *GRPCClient) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	resp, err := m.client.CreateStreamingTableDDL(context.Background(), &proto.CreateStreamingTableRequest{Schema: schema, Table: table, Config: config})
	if err != nil {
		return ""
	}
	return resp.Ddl
}

func (m *GRPCClient) TableExists(ctx context.Context, schema, table string) (bool, error) {
	resp, err := m.client.TableExists(ctx, &proto.TableExistsRequest{Schema: schema, Table: table})
	if err != nil {
		return false, err
	}
	return resp.Exists, nil
}

func (m *GRPCClient) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	resp, err := m.client.CreateIncrementalMergeDDL(context.Background(), &proto.CreateIncrementalMergeRequest{Schema: schema, Table: table, SelectSql: selectSQL, Config: config})
	if err != nil {
		return ""
	}
	return resp.Ddl
}

func (m *GRPCClient) QueryCount(ctx context.Context, sql string) (int, error) {
	resp, err := m.client.QueryCount(ctx, &proto.QueryCountRequest{Sql: sql})
	if err != nil {
		return 0, err
	}
	return int(resp.Count), nil
}

func (m *GRPCClient) Name() string {
	resp, err := m.client.Name(context.Background(), &proto.Empty{})
	if err != nil {
		return ""
	}
	return resp.Name
}

func (m *GRPCClient) QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	// QueryData is excluded from basic gRPC MVP because it returns arbitrary map data,
	// which is complex to serialize efficiently. We'll stub it for plugins.
	return nil, nil
}

// GRPCServer is the gRPC server that GRPCClient talks to.
type GRPCServer struct {
	proto.UnimplementedRunnerPluginServer
	Impl Runner
}

func (m *GRPCServer) Name(ctx context.Context, req *proto.Empty) (*proto.NameResponse, error) {
	return &proto.NameResponse{Name: m.Impl.Name()}, nil
}

func (m *GRPCServer) Exec(ctx context.Context, req *proto.ExecRequest) (*proto.Empty, error) {
	err := m.Impl.Exec(ctx, req.Sql)
	return &proto.Empty{}, err
}

func (m *GRPCServer) CreateSchemaDDL(ctx context.Context, req *proto.CreateSchemaRequest) (*proto.DDLResponse, error) {
	return &proto.DDLResponse{Ddl: m.Impl.CreateSchemaDDL(req.Schema)}, nil
}

func (m *GRPCServer) CreateTableDDL(ctx context.Context, req *proto.CreateTableRequest) (*proto.DDLResponse, error) {
	return &proto.DDLResponse{Ddl: m.Impl.CreateTableDDL(req.Schema, req.Table, req.SelectSql)}, nil
}

func (m *GRPCServer) CreateViewDDL(ctx context.Context, req *proto.CreateViewRequest) (*proto.DDLResponse, error) {
	return &proto.DDLResponse{Ddl: m.Impl.CreateViewDDL(req.Schema, req.Table, req.SelectSql)}, nil
}

func (m *GRPCServer) CreateMaterializedViewDDL(ctx context.Context, req *proto.CreateMaterializedViewRequest) (*proto.DDLResponse, error) {
	return &proto.DDLResponse{Ddl: m.Impl.CreateMaterializedViewDDL(req.Schema, req.Table, req.SelectSql)}, nil
}

func (m *GRPCServer) CreateStreamingTableDDL(ctx context.Context, req *proto.CreateStreamingTableRequest) (*proto.DDLResponse, error) {
	return &proto.DDLResponse{Ddl: m.Impl.CreateStreamingTableDDL(req.Schema, req.Table, req.Config)}, nil
}

func (m *GRPCServer) TableExists(ctx context.Context, req *proto.TableExistsRequest) (*proto.TableExistsResponse, error) {
	exists, err := m.Impl.TableExists(ctx, req.Schema, req.Table)
	return &proto.TableExistsResponse{Exists: exists}, err
}

func (m *GRPCServer) CreateIncrementalMergeDDL(ctx context.Context, req *proto.CreateIncrementalMergeRequest) (*proto.DDLResponse, error) {
	return &proto.DDLResponse{Ddl: m.Impl.CreateIncrementalMergeDDL(req.Schema, req.Table, req.SelectSql, req.Config)}, nil
}

func (m *GRPCServer) QueryCount(ctx context.Context, req *proto.QueryCountRequest) (*proto.QueryCountResponse, error) {
	count, err := m.Impl.QueryCount(ctx, req.Sql)
	return &proto.QueryCountResponse{Count: int64(count)}, err
}
