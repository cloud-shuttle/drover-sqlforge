.PHONY: build wasm cli clean test

build:
	go build -o sqlforge ./cmd/sqlforge

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
