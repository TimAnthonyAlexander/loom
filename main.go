package main

import (
	"fmt"
	"loom/cmd"
	"loom/indexer"
	"os"
)

func init() {
	// Register the embedded ripgrep getter with the indexer package
	indexer.SetEmbeddedRipgrepGetter(GetEmbeddedRipgrepPath)
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
