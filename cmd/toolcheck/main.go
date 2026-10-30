package main

import (
	"context"
	"os"

	"github.com/stalane/toolcheck/internal/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), os.Args, os.Stdout, os.Stderr))
}
