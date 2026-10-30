package check

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/stalane/toolcheck/internal/detect"
	"github.com/stalane/toolcheck/internal/registry"
	"github.com/stalane/toolcheck/internal/sources"
	"github.com/stalane/toolcheck/internal/store"
)

type Fetcher interface {
	Fetch(ctx context.Context, spec string) (sources.Result, error)
}

type Notifier interface {
	Notify(ctx context.Context, tools []store.Tool) int
}

type Engine struct {
	Detector    *detect.Detector
	Fetcher     Fetcher
	Store       *store.Store
	Notifier    Notifier
	Entries     []registry.Tool
	Include     []string
	Exclude     []string
	NodeChannel string
	CacheTTL    time.Duration
	Now         func() time.Time
	Log         *slog.Logger
}

type Result struct {
	Tools    int
	Updates  int
	CheckID  int64
	Notified int
	Errors   int
}

func (e *Engine) Run(ctx context.Context, refresh bool) (Result, error) {
	var result Result
	if e.Detector == nil || e.Store == nil {
		return result, errors.New("check: detector and store are required")
	}

	started := e.now()
	entries := Filter(e.Entries, e.Include, e.Exclude)
	detected := e.Detector.Detect(ctx, entries)
	if err := ctx.Err(); err != nil {
		return result, err
	}

	existing, err := e.Store.ListTools(ctx)
	if err != nil {
		return result, err
	}
	previous := make(map[string]store.Tool, len(existing))
	for _, tool := range existing {
		previous[toolKey(tool.Name, tool.RealPath)] = tool
	}

	type fetched struct {
		result sources.Result
		err    error
	}
	fetches := make(map[string]fetched)
	failedSpecs := make(map[string]bool)

	now := e.now()
	tools := make([]store.Tool, 0, len(detected))
	for _, tool := range detected {
		stored := store.Tool{
			Name:       tool.Name,
			Path:       tool.Path,
			RealPath:   tool.RealPath,
			Family:     tool.Family,
			Version:    tool.Version,
			Origin:     tool.Origin,
			ManagedBy:  tool.ManagedBy,
			Risk:       tool.Risk,
			Source:     tool.Source,
			Active:     tool.Active,
			Writable:   tool.Writable,
			Updateable: tool.Updateable,
		}
		if prior, ok := previous[toolKey(tool.Name, tool.RealPath)]; ok {
			stored.ID = prior.ID
			stored.LatestVersion = prior.LatestVersion
			stored.LatestURL = prior.LatestURL
			stored.LatestPublishedAt = prior.LatestPublishedAt
			stored.LatestSecurity = prior.LatestSecurity
			stored.CheckedAt = prior.CheckedAt
		}

		if e.shouldFetch(stored, refresh, now) {
			spec := stored.Source
			if spec == "node" {
				spec = "node:" + e.nodeChannel()
			}
			var latest fetched
			if cached, ok := fetches[spec]; ok {
				latest = cached
			} else {
				result, err := e.Fetcher.Fetch(ctx, spec)
				latest = fetched{result: result, err: err}
				fetches[spec] = latest
			}
			switch {
			case latest.err == nil:
				stored.LatestVersion = latest.result.Version
				stored.LatestURL = latest.result.URL
				stored.LatestSecurity = latest.result.Security
				if !latest.result.Published.IsZero() {
					published := latest.result.Published
					stored.LatestPublishedAt = &published
				}
				checked := now
				stored.CheckedAt = &checked
			case errors.Is(latest.err, sources.ErrNoSource), errors.Is(latest.err, sources.ErrNoRelease):
				// Not every project publishes releases; this is not a scan failure.
			default:
				if !failedSpecs[spec] {
					failedSpecs[spec] = true
					result.Errors++
				}
				if e.Log != nil {
					e.Log.Warn("release check failed", "tool", stored.Name, "source", spec, "err", latest.err)
				}
			}
		}

		if stored.Active && IsNewer(stored.LatestVersion, stored.Version) && !isSystem(stored.ManagedBy) {
			result.Updates++
		}
		tools = append(tools, stored)
	}

	if err := e.Store.ReplaceTools(ctx, tools); err != nil {
		return result, err
	}

	storedTools, err := e.Store.ListTools(ctx)
	if err != nil {
		return result, err
	}
	result.Tools = len(storedTools)

	if e.Notifier != nil {
		result.Notified = e.Notifier.Notify(ctx, storedTools)
	}

	status := "ok"
	errorText := ""
	if result.Errors > 0 {
		status = "degraded"
		errorText = fmt.Sprintf("%d source errors", result.Errors)
	}
	checkID, err := e.Store.RecordCheck(ctx, store.CheckRecord{
		StartedAt:  started,
		FinishedAt: e.now(),
		Status:     status,
		Tools:      result.Tools,
		Updates:    result.Updates,
		Error:      errorText,
	})
	if err != nil {
		return result, err
	}
	result.CheckID = checkID
	return result, nil
}

func (e *Engine) shouldFetch(tool store.Tool, refresh bool, now time.Time) bool {
	if e.Fetcher == nil || tool.Source == "" || tool.Source == "none" {
		return false
	}
	if refresh || tool.CheckedAt == nil {
		return true
	}
	if e.CacheTTL <= 0 {
		return true
	}
	return now.Sub(*tool.CheckedAt) >= e.CacheTTL
}

func (e *Engine) nodeChannel() string {
	channel := strings.ToLower(strings.TrimSpace(e.NodeChannel))
	if channel == "" {
		return "lts"
	}
	return channel
}

func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func Filter(entries []registry.Tool, include, exclude []string) []registry.Tool {
	includeSet := make(map[string]bool, len(include))
	for _, name := range include {
		includeSet[strings.ToLower(strings.TrimSpace(name))] = true
	}
	excludeSet := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		excludeSet[strings.ToLower(strings.TrimSpace(name))] = true
	}

	filtered := make([]registry.Tool, 0, len(entries))
	for _, entry := range entries {
		name := strings.ToLower(entry.Name)
		if len(includeSet) > 0 && !includeSet[name] {
			continue
		}
		if excludeSet[name] {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func isSystem(managedBy string) bool {
	switch managedBy {
	case "apt", "brew", "nix", "snap", "flatpak":
		return true
	default:
		return false
	}
}

func toolKey(name, realPath string) string {
	return name + "\x00" + realPath
}
