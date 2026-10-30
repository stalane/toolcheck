package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/stalane/toolcheck/internal/store"
	"github.com/stalane/toolcheck/internal/update"
)

func runUpdate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to config.yaml")
	to := flags.String("to", "", "target version (default: latest upstream)")
	dryRun := flags.Bool("dry-run", false, "print the plan without applying it")
	yes := flags.Bool("yes", false, "apply the update")
	force := flags.Bool("force", false, "ignore the minimum release age")
	if err := flags.Parse(normalizeArgs(args)); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: toolcheck update <tool> [--to V] [--dry-run] [--yes] [--force]")
		return 2
	}
	name := flags.Arg(0)

	cfg, _, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 2
	}
	if !cfg.Update.Enabled {
		fmt.Fprintln(stderr, "toolcheck: updates are disabled in config")
		return 1
	}

	st, err := openStore()
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 1
	}
	defer st.Close()

	tools, err := st.ListTools(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 1
	}
	tool, ok := selectTool(tools, name)
	if !ok {
		fmt.Fprintf(stderr, "toolcheck: %s is not installed or was not detected\n", name)
		return 1
	}

	updater := update.New(st)
	plan, err := updater.Plan(tool, *to, cfg.Update.MinReleaseAge.Duration(), *force)
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "%s %s -> %s\n", plan.Name, plan.From, plan.To)
	fmt.Fprintln(stdout, strings.Join(plan.Command, " "))
	if *dryRun || !*yes {
		if !*dryRun {
			fmt.Fprintln(stdout, "pass --yes to apply")
		}
		return 0
	}

	result, err := updater.Apply(ctx, plan)
	if err != nil {
		if result.RolledBack {
			fmt.Fprintf(stderr, "toolcheck: update failed and was rolled back: %v\n", err)
		} else {
			fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		}
		return 1
	}
	if err := st.SetToolVersion(ctx, tool.ID, result.Version); err != nil {
		fmt.Fprintf(stderr, "toolcheck: warning: could not update stored version: %v\n", err)
	}
	fmt.Fprintf(stdout, "updated %s to %s\n", plan.Name, result.Version)
	return 0
}

func selectTool(tools []store.Tool, name string) (store.Tool, bool) {
	var fallback *store.Tool
	for i := range tools {
		tool := &tools[i]
		if !strings.EqualFold(tool.Name, name) {
			continue
		}
		if tool.Active {
			return *tool, true
		}
		if fallback == nil {
			fallback = tool
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return store.Tool{}, false
}

func normalizeArgs(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config" || arg == "-config":
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		case strings.HasPrefix(arg, "-"):
			flags = append(flags, arg)
		default:
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}
