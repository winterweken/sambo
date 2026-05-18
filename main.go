package main

import (
	"fmt"
	"os"
	"sambo/cmd"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	cmd.Version = version
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
