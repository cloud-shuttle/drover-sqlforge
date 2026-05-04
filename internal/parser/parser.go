package parser

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed polyglot.wasm
var polyglotWASM []byte

type Parser struct {
	ctx      context.Context
	runtime  wazero.Runtime
	module   wazero.CompiledModule
	instance api.Module
}

func NewParser(ctx context.Context) (*Parser, error) {
	r := wazero.NewRuntime(ctx)

	// Instantiate WASI, which is required by many Rust-compiled WASM modules.
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	// Compile the WASM module
	var module wazero.CompiledModule
	var err error
	if len(polyglotWASM) > 0 {
		module, err = r.CompileModule(ctx, polyglotWASM)
		if err != nil {
			r.Close(ctx)
			return nil, fmt.Errorf("failed to compile module: %w", err)
		}
	}

	// Instantiate the module (we mock instantiation if the WASM file is empty)
	var instance api.Module
	if module != nil {
		instance, err = r.InstantiateModule(ctx, module, wazero.NewModuleConfig().WithName("polyglot"))
		if err != nil {
			r.Close(ctx)
			return nil, fmt.Errorf("failed to instantiate module: %w", err)
		}
	}

	return &Parser{
		ctx:      ctx,
		runtime:  r,
		module:   module,
		instance: instance,
	}, nil
}

func (p *Parser) Close() error {
	return p.runtime.Close(p.ctx)
}

// ParseToAST simulates extracting a JSON AST from the WASM module.
func (p *Parser) ParseToAST(sql string) (*ASTNode, error) {
	node := &ASTNode{
		Type:  "MockAST",
		Value: sql,
	}
	return node, nil
}

// Transpile simulates translating SQL from one dialect to another using the WASM module.
func (p *Parser) Transpile(sql, fromDialect, toDialect string) (*TranspileResult, error) {
	return &TranspileResult{
		SQL: fmt.Sprintf("-- Transpiled from %s to %s\n%s", fromDialect, toDialect, sql),
	}, nil
}

// ExtractRefs simulates extracting table references structurally from the SQL.
func (p *Parser) ExtractRefs(sql string) ([]string, error) {
	var refs []string
	re := regexp.MustCompile(`(?i)(?:FROM|JOIN)\s+([a-zA-Z0-9_]+)`)
	matches := re.FindAllStringSubmatch(sql, -1)
	for _, m := range matches {
		refs = append(refs, m[1])
	}
	return refs, nil
}

// DetectTimePatterns simulates finding time columns for incremental materializations.
func (p *Parser) DetectTimePatterns(sql string) (string, error) {
	return "event_time", nil
}
