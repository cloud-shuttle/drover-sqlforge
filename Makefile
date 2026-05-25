.PHONY: build wasm cli clean test ui plugins

build: ui
	go build -o sqlforge ./cmd/sqlforge

# Build all standalone gRPC plugin binaries.
# Each binary is placed at the repo root so `sqlforge` can discover them
# alongside the main binary when users run `sqlforge plan` / `sqlforge apply`.
plugins:
	go build -o sqlforge-plugin-duckdb     ./cmd/plugins/sqlforge-plugin-duckdb
	go build -o sqlforge-plugin-databricks ./cmd/plugins/sqlforge-plugin-databricks
	go build -o sqlforge-plugin-snowflake  ./cmd/plugins/sqlforge-plugin-snowflake
	go build -o sqlforge-plugin-doris      ./cmd/plugins/sqlforge-plugin-doris
	go build -o sqlforge-plugin-velodb     ./cmd/plugins/sqlforge-plugin-velodb

ui:
	cd ui && npm ci && npm run build

wasm:
	# Placeholder for future Rust -> WASM build step
	echo "WASM build step"

cli: build
	./sqlforge --help

clean:
	rm -f sqlforge
	rm -rf .sqlforge

test:
	go test ./...

e2e: build
	go test -v ./test/e2e/...

integration: build
	docker compose -f test/integration/docker-compose.yml up -d --wait
	go test -v ./test/integration/...
	docker compose -f test/integration/docker-compose.yml down
