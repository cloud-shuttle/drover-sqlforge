package state

import "time"

type Environment struct {
	Name      string    `json:"name"`
	Schema    string    `json:"schema"`
	CreatedAt time.Time `json:"created_at"`
	IsVirtual bool      `json:"is_virtual"`
	BaseEnv   string    `json:"base_env"`
}

type ModelState struct {
	ModelName      string    `json:"model_name"`
	Fingerprint    string    `json:"fingerprint"`
	LastApplied    time.Time `json:"last_applied"`
	MaterializedAs string    `json:"materialized_as"`
	Environment    string    `json:"environment"`
}
