.PHONY: all build wasm cli clean test ui plugins e2e integration

PLUGINS := duckdb databricks snowflake doris velodb

all: build plugins

build: ui
	go build -o sqlforge ./cmd/sqlforge

# Build all standalone gRPC plugin binaries.
# Each binary is placed at the repo root so `sqlforge` can discover them
# alongside the main binary when users run `sqlforge plan` / `sqlforge apply`.
# The binaries are gitignored: build them for your own OS/arch.
# The DuckDB plugin needs CGO and a C++ toolchain.
plugins:
	@for p in $(PLUGINS); do \
		echo "go build -o sqlforge-plugin-$$p ./cmd/plugins/sqlforge-plugin-$$p"; \
		go build -o sqlforge-plugin-$$p ./cmd/plugins/sqlforge-plugin-$$p || exit 1; \
	done

ui:
	cd ui && npm ci && npm run build

wasm:
	# Placeholder for future Rust -> WASM build step
	echo "WASM build step"

cli: build
	./sqlforge --help

# Consumer smoke test (#59): builds sqlforge + the DuckDB plugin itself and runs
# examples/agentic_retail_2026 end to end against a DuckDB file. Needs CGO.
.PHONY: smoke
smoke:
	go test -tags smoke -count=1 -v ./test/smoke/

clean:
	rm -f sqlforge $(addprefix sqlforge-plugin-,$(PLUGINS))
	rm -rf .sqlforge target

# Some tests exec ./sqlforge and ./sqlforge-plugin-* from the repo root.
test: plugins
	go build -o sqlforge ./cmd/sqlforge
	go test ./...

e2e: build plugins
	go test -v ./test/e2e/...

integration: build
	docker compose -f test/integration/docker-compose.yml up -d --wait
	go test -v ./test/integration/...
	docker compose -f test/integration/docker-compose.yml down

# Quality gate: the same checks CI runs after the test job (#57).
# CRAP (tools/crap, per Go function): every function must score <= the limit
# (30) except legacy functions pinned in quality/crap-baseline.json, which may
# not get worse. Coverage: per-package floors in quality/coverage-floors.json.
.PHONY: coverage quality-gate quality-baseline
coverage:
	TERM=xterm-256color go test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./...

quality-gate:
	@test -f coverage.out || { echo "coverage.out missing: run 'make coverage' first (needs ./sqlforge and plugins, see 'make plugins')"; exit 1; }
	go run ./tools/crap -coverage coverage.out -baseline quality/crap-baseline.json
	python3 scripts/coverage-floors.py --coverage coverage.out --floors quality/coverage-floors.json

# Lower or drop CRAP ceilings after improving tests (never raises or adds).
quality-baseline:
	go run ./tools/crap -coverage coverage.out -baseline quality/crap-baseline.json -update
