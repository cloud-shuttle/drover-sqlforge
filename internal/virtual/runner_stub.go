package virtual

import (
	"context"
	"fmt"
)

type ClickHouseRunnerStub struct{}

func (c *ClickHouseRunnerStub) Exec(ctx context.Context, sql string) error {
	// Simulated execution
	fmt.Printf("[ClickHouse Runner] Executing: %s\n", sql)
	return nil
}
