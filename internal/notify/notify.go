package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gen2brain/beeep"

	"github.com/stalane/toolcheck/internal/check"
	"github.com/stalane/toolcheck/internal/store"
)

type Notifier struct {
	Enabled bool
	Send    func(title, body string) error
	Store   *store.Store
	Log     *slog.Logger
}

func New(enabled bool, st *store.Store, log *slog.Logger) *Notifier {
	return &Notifier{
		Enabled: enabled,
		Send: func(title, body string) error {
			return beeep.Notify(title, body, "")
		},
		Store: st,
		Log:   log,
	}
}

func (n *Notifier) Notify(ctx context.Context, tools []store.Tool) int {
	if !n.Enabled || n.Store == nil {
		return 0
	}
	sent := 0
	for _, tool := range tools {
		if tool.ID <= 0 || !tool.Active || systemManaged(tool.ManagedBy) {
			continue
		}
		if !check.IsNewer(tool.LatestVersion, tool.Version) {
			continue
		}
		notified, err := n.Store.WasNotified(ctx, tool.ID, tool.LatestVersion)
		if err != nil {
			n.warn("could not read notification state", tool, err)
			continue
		}
		if notified {
			continue
		}

		title := fmt.Sprintf("toolcheck: %s %s → %s", tool.Name, tool.Version, tool.LatestVersion)
		body := fmt.Sprintf("%s · %s", tool.Origin, compactPath(tool.Path))
		if err := n.Send(title, body); err != nil {
			n.warn("notification failed", tool, err)
			continue
		}
		if err := n.Store.RecordNotification(ctx, tool.ID, tool.LatestVersion, time.Now()); err != nil {
			n.warn("could not record notification", tool, err)
		}
		sent++
	}
	return sent
}

func (n *Notifier) warn(message string, tool store.Tool, err error) {
	if n.Log != nil {
		n.Log.Warn(message, "tool", tool.Name, "err", err)
	}
}

func systemManaged(managedBy string) bool {
	switch managedBy {
	case "apt", "brew", "nix", "snap", "flatpak":
		return true
	default:
		return false
	}
}

func compactPath(path string) string {
	const max = 100
	if len(path) <= max {
		return path
	}
	return "…" + path[len(path)-max+1:]
}
