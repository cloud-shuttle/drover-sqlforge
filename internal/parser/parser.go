package parser

import (
	"context"
	_ "embed"
	"encoding/json"
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

type transpileRequest struct {
	SQL         string `json:"sql"`
	FromDialect string `json:"from_dialect"`
	ToDialect   string `json:"to_dialect"`
}

// TranspileWASM delegates AST-based SQL transpilation to the embedded Rust WASM module.
// If the WASM function is not exported, it falls back to the mocked Transpile function.
func (p *Parser) TranspileWASM(sql, fromDialect, toDialect string) (*TranspileResult, error) {
	req := transpileRequest{
		SQL:         sql,
		FromDialect: fromDialect,
		ToDialect:   toDialect,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return p.Transpile(sql, fromDialect, toDialect)
	}

	jsonStr, err := p.callWASMString(p.ctx, "transpile_sql", string(reqBytes))
	if err != nil {
		// Fallback to structural/mock parser if WASM function is missing
		return p.Transpile(sql, fromDialect, toDialect)
	}

	if jsonStr == "" {
		return p.Transpile(sql, fromDialect, toDialect)
	}

	var res TranspileResult
	if err := json.Unmarshal([]byte(jsonStr), &res); err != nil {
		return p.Transpile(sql, fromDialect, toDialect)
	}

	return &res, nil
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

// callWASMString is a helper to invoke a WASM function that takes a string and returns a JSON string.
// It assumes the WASM module exports `allocate` and `deallocate` for memory management.
func (p *Parser) callWASMString(ctx context.Context, funcName string, input string) (string, error) {
	if p.instance == nil {
		return "", fmt.Errorf("WASM instance not loaded")
	}

	alloc := p.instance.ExportedFunction("allocate")
	dealloc := p.instance.ExportedFunction("deallocate")
	targetFunc := p.instance.ExportedFunction(funcName)

	if alloc == nil || dealloc == nil || targetFunc == nil {
		return "", fmt.Errorf("required WASM functions not exported (%s)", funcName)
	}

	inputBytes := []byte(input)
	inputSize := uint64(len(inputBytes))

	// Allocate memory
	results, err := alloc.Call(ctx, inputSize)
	if err != nil {
		return "", fmt.Errorf("failed to allocate memory: %w", err)
	}
	ptr := uint32(results[0])

	// Write string to memory
	if !p.instance.Memory().Write(ptr, inputBytes) {
		return "", fmt.Errorf("failed to write to memory")
	}

	// Call function
	results, err = targetFunc.Call(ctx, uint64(ptr), inputSize)
	if err != nil {
		return "", fmt.Errorf("failed to call %s: %w", funcName, err)
	}

	// Read packed result (ptr in high 32 bits, size in low 32 bits)
	packed := results[0]
	outPtr := uint32(packed >> 32)
	outSize := uint32(packed)

	if outSize == 0 {
		return "", nil
	}

	// Read result string
	outBytes, ok := p.instance.Memory().Read(outPtr, outSize)
	if !ok {
		return "", fmt.Errorf("failed to read result from memory")
	}
	
	resultStr := string(outBytes)

	// Free memory
	_, _ = dealloc.Call(ctx, uint64(outPtr), uint64(outSize))

	return resultStr, nil
}

// ExtractColumnLineageWASM delegates column lineage extraction to the embedded Rust WASM module.
func (p *Parser) ExtractColumnLineageWASM(sql string) ([]ColumnMapping, error) {
	jsonStr, err := p.callWASMString(p.ctx, "extract_column_lineage", sql)
	if err != nil {
		return nil, err
	}
	
	if jsonStr == "" {
		return nil, nil
	}
	
	var mappings []ColumnMapping
	if err := json.Unmarshal([]byte(jsonStr), &mappings); err != nil {
		return nil, fmt.Errorf("failed to decode WASM JSON: %w", err)
	}
	return mappings, nil
}
