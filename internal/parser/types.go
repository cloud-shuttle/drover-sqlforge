package parser

type ASTNode struct {
	Type     string      `json:"type"`
	Value    interface{} `json:"value,omitempty"`
	Children []ASTNode   `json:"children,omitempty"`
}

type TranspileResult struct {
	SQL   string `json:"sql"`
	Error string `json:"error,omitempty"`
}
