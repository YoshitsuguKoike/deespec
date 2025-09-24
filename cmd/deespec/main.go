package main

import (
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/interface/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		os.Exit(1)
	}
}
