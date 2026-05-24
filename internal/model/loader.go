package model

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/drover-org/drover-sqlforge/internal/parser"
)

// LoadModels walks the models directory and parses .sql files
func LoadModels(dir string, p *parser.Parser) ([]*Asset, error) {
	var assets []*Asset

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".sql") {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		asset, err := parseFile(path, relPath, p)
		if err != nil {
			return err
		}
		assets = append(assets, asset)
		return nil
	})

	return assets, err
}

func parseFile(path, relPath string, p *parser.Parser) (*Asset, error) {
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

		if key, value, ok := ParseConfigLine(line); ok {
			config[key] = value
		}

		sqlBuilder.WriteString(line + "\n")
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if _, ok := config["schema"]; !ok {
		dirPart := filepath.Dir(relPath)
		if dirPart != "." && dirPart != "" {
			schema := strings.ReplaceAll(dirPart, string(filepath.Separator), "_")
			config["schema"] = schema
		}
	}

	sql := sqlBuilder.String()
	ast, err := p.ParseToAST(sql)
	if err != nil {
		return nil, err
	}

	deps, err := p.ExtractRefs(sql)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSuffix(filepath.Base(path), ".sql")

	return &Asset{
		Name:         name,
		Path:         path,
		Type:         "model",
		Config:       config,
		SQL:          sql,
		AST:          ast,
		Dependencies: deps, // structural table reference extraction
	}, nil
}

// ParseConfigLine extracts metadata from lines formatted like "-- @key: value"
func ParseConfigLine(line string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "-- @") {
		return "", "", false
	}

	parts := strings.SplitN(trimmed[4:], ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	key = strings.TrimSpace(parts[0])
	value = strings.TrimSpace(parts[1])

	// remove trailing comments if any
	if idx := strings.Index(value, "--"); idx != -1 {
		value = strings.TrimSpace(value[:idx])
	}

	return key, value, true
}
