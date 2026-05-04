package model

import "github.com/drover-org/drover-sqlforge/internal/parser"

// Asset represents a parsed model or source
type Asset struct {
	Name         string
	Path         string
	Type         string // "model", "source", "metric"
	Config       map[string]string
	SQL          string
	AST          *parser.ASTNode
	Dependencies []string // Clean structural references
}
