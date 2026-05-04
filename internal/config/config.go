package config

type Config struct {
	Name               string        `yaml:"name"`
	Version            string        `yaml:"version"`
	DefaultEnvironment string        `yaml:"default_environment"`
	Virtual            VirtualConfig `yaml:"virtual"`
	AI                 AIConfig      `yaml:"ai"`
}

type AIConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	Endpoint string `yaml:"endpoint"`
}

type VirtualConfig struct {
	Dialect    string `yaml:"dialect"`
	Connection string `yaml:"connection"`
}
