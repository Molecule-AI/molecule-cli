// molecule-cli — Molecule AI platform CLI
//
// Entry point. Wires cobra root command and runs it.
package main

import (
	"os"

	"github.com/Molecule-AI/molecule-cli/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// Cobra already prints the error; exit with non-zero.
		os.Exit(1)
	}
}
