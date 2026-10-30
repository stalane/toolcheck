package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/stalane/toolcheck/internal/config"
	"github.com/stalane/toolcheck/internal/lock"
	"github.com/stalane/toolcheck/internal/scheduler"
	"github.com/stalane/toolcheck/internal/store"
	"github.com/stalane/toolcheck/internal/update"
	"github.com/stalane/toolcheck/internal/web"
)

func runServe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to config.yaml")
	addr := flags.String("addr", "", "listen address (default from config)")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, _, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 2
	}
	listenAddr := *addr
	if listenAddr == "" {
		listenAddr = cfg.Server.Addr
	}
	if err := config.ValidateLoopbackAddr(listenAddr); err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 2
	}

	dataDir, err := config.DataDir()
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 1
	}
	held, err := lock.Acquire(filepath.Join(dataDir, "toolcheck.lock"))
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 1
	}
	defer held.Release()

	st, err := store.Open(filepath.Join(dataDir, "toolcheck.db"))
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: open store: %v\n", err)
		return 1
	}
	defer st.Close()

	log := logger(stderr)
	engine := newEngine(cfg, st, log)
	updater := update.New(st)

	scanScheduler := scheduler.New(scheduler.Config{
		Interval: cfg.Check.Interval.Duration(),
		Run: func(scanCtx context.Context) error {
			_, err := engine.Run(scanCtx, false)
			return err
		},
		Log: log,
	})
	go func() {
		if err := scanScheduler.Run(ctx); err != nil && ctx.Err() == nil {
			log.Warn("scheduler stopped", "err", err)
		}
	}()

	fmt.Fprintf(stderr, "toolcheck: serving on http://%s\n", listenAddr)
	server := web.New(st, listenAddr, cfg, engine, updater, log)
	if err := server.ListenAndServe(ctx); err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 1
	}
	return 0
}
