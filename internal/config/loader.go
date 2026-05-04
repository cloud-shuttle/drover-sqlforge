package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

func LoadConfig(projectDir string) (*Config, error) {
	// Attempt to load .env file from the project directory (ignore error if it doesn't exist)
	_ = godotenv.Load(filepath.Join(projectDir, ".env"))

	configPath := filepath.Join(projectDir, "sqlforge.yml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Expand environment variables like ${CLICKHOUSE_URL} before unmarshalling
	expandedData := []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err := yaml.Unmarshal(expandedData, &cfg); err != nil {
		return nil, err
	}

	// Apply defaults
	if cfg.AI.Provider == "" {
		cfg.AI.Provider = "ollama"
	}
	if cfg.AI.Endpoint == "" {
		cfg.AI.Endpoint = "http://localhost:11434"
	}
	if cfg.AI.Model == "" {
		cfg.AI.Model = "llama3.2"
	}

	return &cfg, nil
}
