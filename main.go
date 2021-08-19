package main

import (
	"os"

	"github.com/circle-makotom/orbs-sync/cmd"
)

func main() {
	if cmd.Execute() != nil {
		os.Exit(1)
	}
}
