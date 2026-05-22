package parser

type ASTNode struct {
	Type     string      `json:"type"`
	Value    interface{} `json:"value,omitempty"`
	Children []ASTNode   `json:"children,omitempty"`
}

// ColumnRef is an upstream table/model column referenced by a SELECT expression.
type ColumnRef struct {
	Relation string `json:"relation"`
	Column   string `json:"column"`
}

// ColumnMapping maps a model output column to zero or more upstream column refs.
type ColumnMapping struct {
	Output  string      `json:"output"`
	Sources []ColumnRef `json:"sources"`
}

type TranspileResult struct {
	SQL   string `json:"sql"`
	Error string `json:"error,omitempty"`
}
