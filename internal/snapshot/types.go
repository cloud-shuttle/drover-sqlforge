package snapshot

// Definition is a historized snapshot loaded from snapshots/*.sql.
type Definition struct {
	Name   string
	Path   string
	SQL    string
	Config map[string]string
}

// ValidFrom and ValidTo are SCD Type 2 columns injected by SQLForge.
const (
	ValidFrom = "sqlforge_valid_from"
	ValidTo   = "sqlforge_valid_to"
)
