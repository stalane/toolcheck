package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"

	"github.com/stalane/toolcheck/internal/check"
	"github.com/stalane/toolcheck/internal/config"
	"github.com/stalane/toolcheck/internal/detect"
	"github.com/stalane/toolcheck/internal/logging"
	"github.com/stalane/toolcheck/internal/notify"
	"github.com/stalane/toolcheck/internal/registry"
	"github.com/stalane/toolcheck/internal/sources"
	"github.com/stalane/toolcheck/internal/store"
	"github.com/stalane/toolcheck/internal/version"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		usage(stderr)
		return 2
	}

	switch args[1] {
	case "version", "-v", "--version":
		fmt.Fprintln(stdout, version.String())
		return 0
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	case "check":
		return runCheck(ctx, args[2:], stdout, stderr)
	case "list":
		return runList(ctx, args[2:], stdout, stderr)
	case "update":
		return runUpdate(ctx, args[2:], stdout, stderr)
	case "serve":
		return runServe(ctx, args[2:], stdout, stderr)
	case "service":
		return runService(ctx, args[2:], stdout, stderr)
	case "config":
		return runConfig(ctx, args[2:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "toolcheck: unknown command %q\n\n", args[1])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: toolcheck <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  version    print the toolcheck version")
	fmt.Fprintln(w, "  check      detect installed tools and check upstream releases")
	fmt.Fprintln(w, "  list       list detected tools and their state")
	fmt.Fprintln(w, "  update     plan or apply a safe update")
	fmt.Fprintln(w, "  serve      serve the local dashboard on 127.0.0.1")
	fmt.Fprintln(w, "  service    install or uninstall the systemd user unit")
	fmt.Fprintln(w, "  config     print the config file path")
	fmt.Fprintln(w, "  help       print this help")
}

func loadConfig(path string) (config.Config, string, error) {
	complete, err := config.Complete(path)
	if err != nil {
		return config.Config{}, "", err
	}
	cfg, err := config.Load(complete)
	if err != nil {
		return config.Config{}, complete, err
	}
	return cfg, complete, nil
}

func openStore() (*store.Store, error) {
	dataDir, err := config.DataDir()
	if err != nil {
		return nil, err
	}
	return store.Open(filepath.Join(dataDir, "toolcheck.db"))
}

func newEngine(cfg config.Config, st *store.Store, log *slog.Logger) *check.Engine {
	detector := detect.New(detect.Options{Timeout: cfg.Check.Timeout.Duration()})
	fetcher := sources.New("toolcheck/" + version.Version)
	return &check.Engine{
		Detector:    detector,
		Fetcher:     fetcher,
		Store:       st,
		Notifier:    notify.New(cfg.Notify.Desktop, st, log),
		Entries:     registry.All(),
		Include:     cfg.Tools.Include,
		Exclude:     cfg.Tools.Exclude,
		NodeChannel: cfg.NodeChannel(),
		CacheTTL:    cfg.Check.CacheTTL.Duration(),
		Log:         log,
	}
}

func logger(stderr io.Writer) *slog.Logger {
	return logging.New(stderr, slog.LevelInfo)
}
