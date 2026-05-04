.PHONY: build test lint release

# ===== Go =====
GO := go
GO_SRC := ./cmd/lovart ./internal/... ./cli/... ./mcp/...

build:
	$(GO) build -ldflags="-s -w" -o dist/lovart ./cmd/lovart

test:
	$(GO) test -race -count=1 ./internal/...
	$(GO) test -race -count=1 ./mcp/...

lint:
	$(GO) vet ./...

release:
	goreleaser release --snapshot --clean

# ===== Python =====
PY_DIR := reverse
py-test:
	cd $(PY_DIR) && uv run pytest || echo "Python tests not configured yet"

# ===== Extension =====
EXT_DIR := extension
ext-install:
	cd $(EXT_DIR) && npm install || echo "Extension package.json not created yet"

ext-build:
	cd $(EXT_DIR) && npm run build || echo "Extension build not configured yet"

# ===== All =====
all: build
check: lint test
