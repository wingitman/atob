## atob — Makefile
## Targets: build  install  uninstall  clean  release

BINARY     := atob
CMD        := .
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w \
              -X main.version=$(VERSION) \
              -X main.buildTime=$(BUILD_TIME)

INSTALL_DIR ?= $(HOME)/.local/bin
DIST        := dist

.PHONY: all build install uninstall clean release help

all: build

## build: compile atob for the current platform
build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) $(CMD)

## install: build and copy binary to INSTALL_DIR (default ~/.local/bin)
install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
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
