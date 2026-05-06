.PHONY: build test lint release

# ===== Go =====
GO := go

build:
	$(GO) build -ldflags="-s -w" -o dist/lovart ./cmd/lovart

test:
	$(GO) test -race -count=1 ./...

lint:
	$(GO) vet ./...

release:
	goreleaser release --snapshot --clean

# ===== Python reverse =====
PY_DIR := reverse
py-setup:
	cd $(PY_DIR) && uv sync --extra reverse

py-test:
	cd $(PY_DIR) && uv run --extra dev pytest

# One-command browser capture session.
# Starts mitmproxy + Chrome, captures Lovart traffic to captures/.
# Press Ctrl-C when done. Then run: make extract FILE=captures/<latest>.json
capture:
	cd $(PY_DIR) && uv run python -m lovart_reverse.cli.main start

# Extract credentials (cookie, token, project_id) from a capture file.
# Usage: make extract FILE=captures/lovart-request.json
extract:
	cd $(PY_DIR) && uv run python -m lovart_reverse.cli.main extract $(FILE)

# ===== Extension =====
EXT_DIR := extension
ext-install:
	cd $(EXT_DIR) && npm install || echo "Extension package.json not created yet"

ext-build:
	cd $(EXT_DIR) && npm run build || echo "Extension build not configured yet"

# ===== All =====
all: build
check: lint test
