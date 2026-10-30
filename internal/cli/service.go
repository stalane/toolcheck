package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/stalane/toolcheck/internal/service"
)

func runService(_ context.Context, args []string, stdout, stderr io.Writer) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintln(stderr, "toolcheck: service is only supported on Linux")
		return 1
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: toolcheck service install|uninstall")
		return 2
	}

	switch args[0] {
	case "install":
		binary, err := os.Executable()
		if err != nil {
			fmt.Fprintf(stderr, "toolcheck: %v\n", err)
			return 1
		}
		path, err := service.Install(binary)
		if err != nil {
			fmt.Fprintf(stderr, "toolcheck: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "installed %s\n", path)
		return 0
	case "uninstall":
		if err := service.Uninstall(); err != nil {
			fmt.Fprintf(stderr, "toolcheck: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "uninstalled systemd user unit")
		return 0
	default:
		fmt.Fprintln(stderr, "usage: toolcheck service install|uninstall")
		return 2
	}
}
