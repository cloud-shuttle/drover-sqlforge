package config

type Config struct {
	Name               string             `yaml:"name"`
	Version            string             `yaml:"version"`
	DefaultEnvironment string             `yaml:"default_environment"`
	ClickHouse         ClickHouseConfig   `yaml:"clickhouse"`
	AI                 AIConfig           `yaml:"ai"`
	Virtual            VirtualConfig      `yaml:"virtual"`
}

type ClickHouseConfig struct {
	Connection string `yaml:"connection"`
}

type AIConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	Endpoint string `yaml:"endpoint"`
}

type VirtualConfig struct {
	DefaultType           string `yaml:"default_type"`
	ClickHouseCloneEngine string `yaml:"clickhouse_clone_engine"`
}
