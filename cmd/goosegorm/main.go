package main

import (
	"os"

	"github.com/pankajredekar/goosegorm/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
