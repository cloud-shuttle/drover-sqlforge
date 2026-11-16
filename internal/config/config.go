package config

type Config struct {
	Name               string        `yaml:"name"`
	Version            string        `yaml:"version"`
	DefaultEnvironment string        `yaml:"default_environment"`
	Virtual            VirtualConfig `yaml:"virtual"`
	AI                 AIConfig      `yaml:"ai"`
	// ProtectedEnvironments lists environments that agents may not change
	// through MCP apply_change unless the server is started with
	// --allow-protected-apply. Nil (unset) means DefaultProtectedEnvironments;
	// an explicit empty list protects nothing.
	ProtectedEnvironments *[]string `yaml:"protected_environments"`
}

// DefaultProtectedEnvironments applies when sqlforge.yml does not set
// protected_environments.
var DefaultProtectedEnvironments = []string{"prod"}

// Protected returns the effective protected environment list.
func (c *Config) Protected() []string {
	if c == nil || c.ProtectedEnvironments == nil {
		return append([]string(nil), DefaultProtectedEnvironments...)
	}
	return append([]string(nil), (*c.ProtectedEnvironments)...)
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
