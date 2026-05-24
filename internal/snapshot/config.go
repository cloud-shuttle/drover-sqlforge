package snapshot

import (
	"fmt"
	"strings"
)

// ResolvedConfig holds validated snapshot settings.
type ResolvedConfig struct {
	Strategy  string
	UniqueKey string
	UpdatedAt string
	CheckCols []string
}

// ResolveConfig validates snapshot config and applies defaults.
func ResolveConfig(def *Definition) (ResolvedConfig, error) {
	cfg := ResolvedConfig{
		Strategy: strings.ToLower(strings.TrimSpace(def.Config["strategy"])),
	}
	if cfg.Strategy == "" {
		cfg.Strategy = "timestamp"
	}

	cfg.UniqueKey = strings.TrimSpace(def.Config["unique_key"])
	if cfg.UniqueKey == "" {
		cfg.UniqueKey = strings.TrimSpace(def.Config["grain"])
	}
	if cfg.UniqueKey == "" {
		return cfg, fmt.Errorf("snapshot %s: unique_key or grain is required", def.Name)
	}

	cfg.UpdatedAt = strings.TrimSpace(def.Config["updated_at"])
	if cfg.Strategy == "timestamp" && cfg.UpdatedAt == "" {
		return cfg, fmt.Errorf("snapshot %s: updated_at is required for timestamp strategy", def.Name)
	}

	if cfg.Strategy == "check" {
		cols := strings.Split(def.Config["check_cols"], ",")
		for _, c := range cols {
			c = strings.TrimSpace(c)
			if c != "" {
				cfg.CheckCols = append(cfg.CheckCols, c)
			}
		}
		if len(cfg.CheckCols) == 0 {
			return cfg, fmt.Errorf("snapshot %s: check_cols is required and must not be empty for check strategy", def.Name)
		}
	}

	switch cfg.Strategy {
	case "timestamp", "check":
		return cfg, nil
	default:
		return cfg, fmt.Errorf("snapshot %s: unknown strategy %q", def.Name, cfg.Strategy)
	}
}
