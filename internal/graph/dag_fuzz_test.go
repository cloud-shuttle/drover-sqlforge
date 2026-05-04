package graph

import (
	"testing"
	"github.com/drover-org/drover-sqlforge/internal/model"
)

// FuzzDAGBuild checks that no sequence of dependencies causes a panic
func FuzzDAGBuild(f *testing.F) {
	// Provide a seed corpus of byte slices (representing random edges)
	f.Add([]byte{0, 1, 1, 2, 2, 0}) // Triangle cycle
	f.Add([]byte{0, 1, 1, 2, 2, 3}) // Linear
	f.Add([]byte{})                 // Empty

	f.Fuzz(func(t *testing.T, data []byte) {
		dag := NewDAG()
		
		// If data length is odd, drop the last byte to get pairs of edges
		length := len(data)
		if length%2 != 0 {
			length--
		}

		var assets []*model.Asset
		created := make(map[byte]bool)

		for i := 0; i < length; i += 2 {
			from := data[i]
			to := data[i+1]
			
			if !created[from] {
				assets = append(assets, &model.Asset{
					Name: string([]byte{from}),
				})
				created[from] = true
			}
			
			// Find the asset and add dependency
			for _, a := range assets {
				if a.Name == string([]byte{from}) {
					a.Dependencies = append(a.Dependencies, string([]byte{to}))
					break
				}
			}
			
			if !created[to] {
				assets = append(assets, &model.Asset{
					Name: string([]byte{to}),
				})
				created[to] = true
			}
		}

		// Ensure Build doesn't panic. Cycle detection might return an error, which is fine.
		_ = dag.Build(assets)
		
		// Ensure TopologicalSort doesn't panic.
		_, _ = dag.TopologicalSort()
	})
}
