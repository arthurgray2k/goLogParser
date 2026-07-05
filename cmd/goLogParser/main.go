package main

import (
	"os"

	"github.com/arthurgray2k/goLogParser/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
