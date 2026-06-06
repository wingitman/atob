## atob — Makefile
## Targets: build  install  uninstall  clean  release

BINARY     := atob
CMD        := .
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse HEAD 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w \
               -X main.version=$(VERSION) \
               -X main.buildTime=$(BUILD_TIME) \
               -X github.com/wingitman/atob/internal/version.Commit=$(COMMIT)

INSTALL_DIR ?= $(HOME)/.local/bin
DIST        := dist
RELEASES_DIR := releases

.PHONY: all build build-all install uninstall clean release help

all: build

## build: compile atob for the current platform
build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) $(CMD)

build-all:
	@mkdir -p $(RELEASES_DIR)/linux/amd64 $(RELEASES_DIR)/linux/arm64 $(RELEASES_DIR)/darwin/amd64 $(RELEASES_DIR)/darwin/arm64 $(RELEASES_DIR)/windows
	GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(RELEASES_DIR)/linux/amd64/$(BINARY) $(CMD)
	GOOS=linux   GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(RELEASES_DIR)/linux/arm64/$(BINARY) $(CMD)
	GOOS=darwin  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(RELEASES_DIR)/darwin/amd64/$(BINARY) $(CMD)
	GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(RELEASES_DIR)/darwin/arm64/$(BINARY) $(CMD)
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(RELEASES_DIR)/windows/$(BINARY).exe $(CMD)
	@echo "Pre-built binaries written to $(RELEASES_DIR)/"

## install: build and copy binary to INSTALL_DIR (default ~/.local/bin)
install:
	mkdir -p $(INSTALL_DIR)
	@if command -v go >/dev/null 2>&1; then \
		echo "==> Go found - building atob from source..."; \
		go build -ldflags="$(LDFLAGS)" -o $(BINARY) $(CMD) || exit 1; \
		cp $(BINARY) $(INSTALL_DIR)/$(BINARY); \
		echo "    Built and installed from source."; \
	else \
		echo "==> Go not found - installing pre-built binary from releases/..."; \
		OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
		ARCH=$$(uname -m); \
		case "$$ARCH" in x86_64|amd64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; *) echo "ERROR: Unsupported architecture: $$ARCH"; exit 1 ;; esac; \
		if [ "$$OS" = "darwin" ]; then RELEASE_BIN="$(RELEASES_DIR)/darwin/$$ARCH/$(BINARY)"; elif [ "$$OS" = "linux" ]; then RELEASE_BIN="$(RELEASES_DIR)/linux/$$ARCH/$(BINARY)"; else echo "ERROR: Unsupported OS: $$OS"; exit 1; fi; \
		if [ ! -f "$$RELEASE_BIN" ]; then echo "ERROR: Pre-built binary not found at $$RELEASE_BIN"; echo "       Please install Go (https://go.dev/dl/) and re-run, or ask a developer to run 'make build-all' and commit the releases/ folder."; exit 1; fi; \
		cp "$$RELEASE_BIN" $(INSTALL_DIR)/$(BINARY); \
		echo "    Installed pre-built binary."; \
	fi
	chmod +x $(INSTALL_DIR)/$(BINARY)
	@echo "✓ atob installed → $(INSTALL_DIR)/$(BINARY)"
	@if ! echo "$$PATH" | grep -q "$(INSTALL_DIR)"; then \
		echo ""; \
		echo "  ⚠  $(INSTALL_DIR) is not in your \$$PATH."; \
		echo "     Add to your shell profile:"; \
		echo "       export PATH=\"$(INSTALL_DIR):\$$PATH\""; \
		echo ""; \
	fi

## uninstall: remove binary from INSTALL_DIR
uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "✓ atob removed from $(INSTALL_DIR)"

## clean: remove local build artefacts
clean:
	rm -f $(BINARY)
	rm -rf $(DIST)/

## release: cross-compile for all supported platforms into dist/
release: clean
	mkdir -p $(DIST)
	GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-amd64   $(CMD)
	GOOS=linux   GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-arm64   $(CMD)
	GOOS=darwin  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)-darwin-amd64  $(CMD)
	GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)-darwin-arm64  $(CMD)
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)-windows-amd64.exe $(CMD)
	@echo ""
	@echo "Release binaries:"
	@ls -lh $(DIST)/

## tui: launch the interactive TUI (shorthand for 'atob' with no args)
tui: build
	./atob

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/^## /  make /'
