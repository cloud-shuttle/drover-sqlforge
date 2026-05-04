package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/drover-org/drover-sqlforge/internal/parser"
)

// GenerateFingerprint creates a stable hash from a normalized AST and config map.
func GenerateFingerprint(ast *parser.ASTNode, config map[string]string) (string, error) {
	type payload struct {
		AST    *parser.ASTNode   `json:"ast"`
		Config map[string]string `json:"config"`
	}

	p := payload{
		AST:    ast,
		Config: config,
	}

	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
