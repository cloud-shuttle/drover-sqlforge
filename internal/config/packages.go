package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PackageDef defines an external dependency.
type PackageDef struct {
	Git      string `yaml:"git"`
	Revision string `yaml:"revision"`
}

// PackageConfig defines the structure of packages.yml.
type PackageConfig struct {
	Packages []PackageDef `yaml:"packages"`
}

// LoadPackagesConfig parses the packages.yml file in the given project directory.
// Returns an empty config and no error if the file does not exist.
func LoadPackagesConfig(projectDir string) (*PackageConfig, error) {
	path := filepath.Join(projectDir, "packages.yml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &PackageConfig{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg PackageConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
