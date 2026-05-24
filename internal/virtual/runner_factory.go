package virtual

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hashicorp/go-plugin"
)

func NewRunner(dialect string, dsn string) (Runner, error) {
	dialect = strings.ToLower(strings.TrimSpace(dialect))

	switch dialect {
	case "clickhouse", "":
		return NewClickHouseRunner(dsn)
	case "postgres":
		return NewPostgresRunner(dsn)
	case "doris":
		return NewDorisRunner(dsn)
	case "velodb":
		return NewVeloDBRunner(dsn)
	default:
		// Attempt to load as a plugin
		return loadRunnerPlugin(dialect, dsn)
	}
}

func loadRunnerPlugin(dialect string, dsn string) (Runner, error) {
	cmdName := fmt.Sprintf("sqlforge-plugin-%s", dialect)
	if _, err := os.Stat(cmdName); err == nil {
		cmdName = fmt.Sprintf("./%s", cmdName)
	} else if _, err := os.Stat(fmt.Sprintf("../../%s", cmdName)); err == nil {
		cmdName = fmt.Sprintf("../../%s", cmdName)
	} else if _, err := os.Stat(fmt.Sprintf("../../bin/%s", cmdName)); err == nil {
		cmdName = fmt.Sprintf("../../bin/%s", cmdName)
	}
	cmd := exec.Command(cmdName)
	cmd.Env = append(os.Environ(), fmt.Sprintf("SQLFORGE_PLUGIN_DSN=%s", dsn))

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"runner": &RunnerGRPCPlugin{},
		},
		Cmd:              cmd,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to start plugin %s: %w", cmdName, err)
	}

	raw, err := rpcClient.Dispense("runner")
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense runner plugin %s: %w", cmdName, err)
	}

	return raw.(Runner), nil
}
