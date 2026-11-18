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
	MCP                   MCPConfig     `yaml:"mcp"`
}

// MCPConfig is the optional governance block. The AI virtual key is not a
// field here or on AIConfig: it is read only from SQLFORGE_AI_VIRTUAL_KEY.
type MCPConfig struct {
	Auth  MCPAuthConfig `yaml:"auth"`
	Hooks []string      `yaml:"hooks"`
}

type MCPAuthConfig struct {
	Mode        string `yaml:"mode"`
	Issuer      string `yaml:"issuer"`
	Audience    string `yaml:"audience"`
	TokenHeader string `yaml:"token_header"`
	RequiredOrg string `yaml:"required_org"`
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
