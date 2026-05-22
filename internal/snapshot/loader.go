package snapshot

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/drover-org/drover-sqlforge/internal/model"
)

// LoadSnapshots walks snapshots/ and parses .sql snapshot definitions.
func LoadSnapshots(dir string) ([]*Definition, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	var defs []*Definition
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".sql") {
			return nil
		}

		def, err := parseFile(path)
		if err != nil {
			return err
		}
		defs = append(defs, def)
		return nil
	})
	return defs, err
}

func parseFile(path string) (*Definition, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := make(map[string]string)
	var sqlBuilder strings.Builder

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if key, value, ok := model.ParseConfigLine(line); ok {
			config[key] = value
		}
		sqlBuilder.WriteString(line + "\n")
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	name := strings.TrimSuffix(filepath.Base(path), ".sql")
	return &Definition{
		Name:   name,
		Path:   path,
		SQL:    strings.TrimSpace(sqlBuilder.String()),
		Config: config,
	}, nil
}

// FilterByNames returns defs matching names, or all if names is empty.
func FilterByNames(defs []*Definition, names []string) ([]*Definition, error) {
	if len(names) == 0 {
		return defs, nil
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	var out []*Definition
	for _, d := range defs {
		if want[d.Name] {
			out = append(out, d)
			delete(want, d.Name)
		}
	}
	if len(want) > 0 {
		missing := make([]string, 0, len(want))
		for n := range want {
			missing = append(missing, n)
		}
		return nil, fmt.Errorf("snapshot(s) not found: %s", strings.Join(missing, ", "))
	}
	return out, nil
}
