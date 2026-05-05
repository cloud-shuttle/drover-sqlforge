.PHONY: build wasm cli clean test ui

build: ui
	go build -o sqlforge ./cmd/sqlforge

ui:
	cd ui && npm install && npm run build

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
