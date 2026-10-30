package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/stalane/toolcheck/internal/config"
)

func runConfig(_ context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) > 1 || (len(args) == 1 && args[0] != "path") {
		fmt.Fprintln(stderr, "usage: toolcheck config path")
		return 2
	}
	path, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, path)
	return 0
}
