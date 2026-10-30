package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/stalane/toolcheck/internal/present"
	"github.com/stalane/toolcheck/internal/store"
)

func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to config.yaml")
	refresh := flags.Bool("refresh", false, "ignore the cached upstream result")
	asJSON := flags.Bool("json", false, "print the check result as JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, _, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 2
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 1
	}
	defer st.Close()

	engine := newEngine(cfg, st, logger(stderr))
	result, err := engine.Run(ctx, *refresh)
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 1
	}

	if *asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(stderr, "toolcheck: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "checked %d tools; %d updates available; %d notifications\n", result.Tools, result.Updates, result.Notified)
	if result.Errors > 0 {
		fmt.Fprintf(stderr, "toolcheck: %d source errors (see activity)\n", result.Errors)
	}

	tools, err := st.ListTools(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 1
	}
	printTools(stdout, tools, true, true)
	return 0
}

func runList(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to config.yaml")
	asJSON := flags.Bool("json", false, "print tools as JSON")
	all := flags.Bool("all", false, "include inactive duplicate installs")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, _, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "toolcheck: %v\n", err)
		return 2
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

	filtered := make([]store.Tool, 0, len(tools))
	for _, tool := range tools {
		if !*all && !tool.Active {
			continue
		}
		if cfg.Tools.HideOSManaged && present.SystemManaged(tool) {
			continue
		}
		filtered = append(filtered, tool)
	}

	if *asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(filtered); err != nil {
			fmt.Fprintf(stderr, "toolcheck: %v\n", err)
			return 1
		}
		return 0
	}

	if len(filtered) == 0 {
		fmt.Fprintln(stdout, "no tools detected; run toolcheck check")
		return 0
	}
	printTools(stdout, filtered, false, false)
	return 0
}

func printTools(w io.Writer, tools []store.Tool, onlyUpdates, activeOnly bool) {
	header := fmt.Sprintf("%-14s %-18s %-18s %-18s %-10s %-8s", "TOOL", "INSTALLED", "LATEST", "ORIGIN", "MANAGED", "STATE")
	fmt.Fprintln(w, header)
	rows := 0
	for _, tool := range tools {
		if activeOnly && !tool.Active {
			continue
		}
		if onlyUpdates && (!present.UpdateAvailable(tool) || present.SystemManaged(tool)) {
			continue
		}
		state := present.State(tool)
		latest := tool.LatestVersion
		if latest == "" {
			latest = "—"
		}
		fmt.Fprintf(w, "%-14s %-18s %-18s %-18s %-10s %-8s\n",
			tool.Name, tool.Version, latest, tool.Origin, tool.ManagedBy, state)
		rows++
	}
	if onlyUpdates && rows == 0 {
		fmt.Fprintln(w, "all detected tools are current")
	}
}

func trimError(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}
