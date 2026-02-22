package main

import (
	"os"

	"github.com/miltonparedes/kitmux/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
