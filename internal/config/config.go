package config

type Config struct {
	Name               string `yaml:"name"`
	Version            string `yaml:"version"`
	DefaultEnvironment string `yaml:"default_environment"`
	// ProtectedEnvironments lists environments where MCP apply_change is
	// refused unless the server is started with --allow-protected-apply.
	// Nil (field omitted) means prod. An explicit empty list protects nothing.
	ProtectedEnvironments []string      `yaml:"protected_environments"`
	Virtual               VirtualConfig `yaml:"virtual"`
	AI                    AIConfig      `yaml:"ai"`
}

// ProtectedSet is the set of environment names that refuse MCP apply_change.
// A nil map means the default (prod). An empty non-nil map protects nothing.
func (c *Config) ProtectedSet() map[string]struct{} {
	if c == nil || c.ProtectedEnvironments == nil {
		return nil
	}
	set := make(map[string]struct{}, len(c.ProtectedEnvironments))
	for _, name := range c.ProtectedEnvironments {
		if name == "" {
			continue
		}
		set[name] = struct{}{}
	}
	return set
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
