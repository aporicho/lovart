.PHONY: build test lint release

# ===== Go =====
GO := go
VERSION ?= snapshot
LDFLAGS := -s -w -X github.com/aporicho/lovart/internal/version.Version=$(VERSION)

build:
	$(GO) build -ldflags="-s -w" -o dist/lovart ./cmd/lovart

test:
	$(GO) test -race -count=1 ./...

lint:
	$(GO) vet ./...

release:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o dist/lovart-macos-arm64 ./cmd/lovart
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o dist/lovart-linux-x64 ./cmd/lovart
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o dist/lovart-windows-x64.exe ./cmd/lovart
	cd extension && zip -qr ../dist/lovart-connector-extension.zip manifest.json src
	cp packaging/install/install.sh dist/install.sh
	cp packaging/install/install.ps1 dist/install.ps1
	chmod +x dist/install.sh
	cd dist && (sha256sum lovart-macos-arm64 lovart-linux-x64 lovart-windows-x64.exe lovart-connector-extension.zip install.sh install.ps1 2>/dev/null || shasum -a 256 lovart-macos-arm64 lovart-linux-x64 lovart-windows-x64.exe lovart-connector-extension.zip install.sh install.ps1) > SHA256SUMS

# ===== Extension =====
EXT_DIR := extension
ext-install:
	cd $(EXT_DIR) && npm install || echo "Extension package.json not created yet"

ext-build:
	mkdir -p dist
	cd $(EXT_DIR) && zip -qr ../dist/lovart-connector-extension.zip manifest.json src

# ===== All =====
all: build
check: lint test
