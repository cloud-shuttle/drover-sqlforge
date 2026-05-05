package semantic

import (
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

type Metric struct {
	Name        string   `yaml:"name"`
	Expression  string   `yaml:"expression"`
	Model       string   `yaml:"model"`
	Dimensions  []string `yaml:"dimensions"`
	Materialize bool     `yaml:"materialize"`
}

type Graph struct {
	Metrics []Metric `yaml:"metrics"`
}

func LoadMetrics(projectDir string) (*Graph, error) {
	pattern := filepath.Join(projectDir, "models", "semantic", "*.yml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	graph := &Graph{}

	for _, match := range matches {
		data, err := os.ReadFile(match)
		if err != nil {
			return nil, err
		}

		var fileGraph Graph
		if err := yaml.Unmarshal(data, &fileGraph); err != nil {
			return nil, err
		}

		graph.Metrics = append(graph.Metrics, fileGraph.Metrics...)
	}

	return graph, nil
}

func (g *Graph) FindMetric(name string) *Metric {
	for _, m := range g.Metrics {
		if m.Name == name {
			return &m
		}
	}
	return nil
}
