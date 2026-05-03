package main

import "github.com/wingitman/atob/cmd"

// version and buildTime are set at compile time via -ldflags:
//
//	-X main.version=$(git describe --tags --always --dirty)
//	-X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cmd.SetVersion(version, buildTime)
	cmd.Execute()
}
