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
