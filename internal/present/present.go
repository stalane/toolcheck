package present

import (
	"github.com/stalane/toolcheck/internal/check"
	"github.com/stalane/toolcheck/internal/store"
)

const (
	StateUpdate  = "update"
	StateCurrent = "current"
	StateSystem  = "system"
	StateManual  = "manual"
)

func UpdateAvailable(tool store.Tool) bool {
	return check.IsNewer(tool.LatestVersion, tool.Version)
}

func SystemManaged(tool store.Tool) bool {
	switch tool.ManagedBy {
	case "apt", "brew", "nix", "snap", "flatpak":
		return true
	default:
		return false
	}
}

func State(tool store.Tool) string {
	if SystemManaged(tool) {
		return StateSystem
	}
	if tool.ManagedBy == "manual" || tool.ManagedBy == "unknown" {
		return StateManual
	}
	if UpdateAvailable(tool) {
		return StateUpdate
	}
	return StateCurrent
}

func Label(state string) string {
	switch state {
	case StateUpdate:
		return "update"
	case StateSystem:
		return "system"
	case StateManual:
		return "manual"
	default:
		return "current"
	}
}
