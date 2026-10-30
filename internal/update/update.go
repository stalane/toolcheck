package update

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/stalane/toolcheck/internal/check"
	"github.com/stalane/toolcheck/internal/detect"
	"github.com/stalane/toolcheck/internal/registry"
	"github.com/stalane/toolcheck/internal/store"
)

var (
	ErrUnsupported = errors.New("update: this tool has no safe adapter")
	ErrNotAllowed  = errors.New("update: tool is not user-managed")
	ErrNotWritable = errors.New("update: install directory is not writable")
	ErrTooNew      = errors.New("update: target release is younger than the minimum age")
	ErrNotNewer    = errors.New("update: target version is not newer than the installed version")
)

type Plan struct {
	ToolID     int64
	Name       string
	Path       string
	From       string
	To         string
	Command    []string
	Rollback   []string
	LatestOnly bool
	Published  time.Time
	MinAge     time.Duration
	Force      bool
}

type Result struct {
	Version    string
	Output     string
	RolledBack bool
}

type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s %s: %w: %s", filepath.Base(name), strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

type Updater struct {
	Runner  Runner
	Store   *store.Store
	Now     func() time.Time
	Access  func(string) bool
	Probe   func(ctx context.Context, plan Plan) (string, error)
	Timeout time.Duration
}

func New(st *store.Store) *Updater {
	return &Updater{
		Runner:  ExecRunner{},
		Store:   st,
		Now:     time.Now,
		Access:  writableDir,
		Timeout: 10 * time.Second,
	}
}

func (u *Updater) Plan(tool store.Tool, target string, minAge time.Duration, force bool) (Plan, error) {
	entry, ok := registry.ByName(tool.Name)
	if !ok || entry.Update == nil {
		return Plan{}, fmt.Errorf("%w: %s", ErrUnsupported, tool.Name)
	}
	if tool.ManagedBy != "self" || !tool.Updateable {
		return Plan{}, fmt.Errorf("%w: %s is managed by %s", ErrNotAllowed, tool.Name, tool.ManagedBy)
	}
	if !u.access()(filepath.Dir(tool.RealPath)) {
		return Plan{}, fmt.Errorf("%w: %s", ErrNotWritable, filepath.Dir(tool.RealPath))
	}
	if !force && minAge > 0 && tool.LatestPublishedAt != nil {
		if u.now().Sub(*tool.LatestPublishedAt) < minAge {
			return Plan{}, fmt.Errorf("%w: %s %s was published %s ago", ErrTooNew, tool.Name, tool.LatestVersion, u.now().Sub(*tool.LatestPublishedAt).Round(time.Hour))
		}
	}

	to := strings.TrimSpace(target)
	if entry.Update.LatestOnly {
		to = tool.LatestVersion
	} else if to == "" {
		to = tool.LatestVersion
	}
	if to == "" {
		return Plan{}, fmt.Errorf("update: no target version known for %s", tool.Name)
	}
	if !check.IsNewer(to, tool.Version) {
		return Plan{}, fmt.Errorf("%w: %s → %s", ErrNotNewer, tool.Version, to)
	}

	command, rollback := commandFor(entry.Update.Kind, tool.Path, tool.Version, to)
	plan := Plan{
		ToolID:     tool.ID,
		Name:       tool.Name,
		Path:       tool.Path,
		From:       tool.Version,
		To:         to,
		Command:    command,
		Rollback:   rollback,
		LatestOnly: entry.Update.LatestOnly,
		MinAge:     minAge,
		Force:      force,
	}
	if tool.LatestPublishedAt != nil {
		plan.Published = *tool.LatestPublishedAt
	}
	return plan, nil
}

func (u *Updater) Apply(ctx context.Context, plan Plan) (Result, error) {
	if len(plan.Command) == 0 {
		return Result{}, errors.New("update: empty plan")
	}

	before, err := u.probe(ctx, plan)
	if err != nil {
		return Result{}, fmt.Errorf("update: pre-flight version check failed: %w", err)
	}
	if before != "" && plan.From != "" && before != plan.From {
		return Result{}, fmt.Errorf("update: %s changed from %s to %s since the check; refusing", plan.Name, plan.From, before)
	}

	started := u.now()
	output, runErr := u.runner().Run(ctx, filepath.Dir(plan.Path), plan.Command[0], plan.Command[1:]...)
	if runErr != nil {
		u.record(ctx, plan, "failed", output, started)
		return Result{Output: output}, runErr
	}

	after, verifyErr := u.probe(ctx, plan)
	if verifyErr == nil && after != "" && !atLeast(after, plan.To) {
		verifyErr = fmt.Errorf("update: verified version %s is below target %s", after, plan.To)
	}
	if verifyErr != nil || after == "" {
		if verifyErr == nil {
			verifyErr = errors.New("update: could not verify the new version")
		}
		result := Result{Output: output}
		status := "failed"
		if len(plan.Rollback) > 0 {
			rollbackOutput, rollbackErr := u.runner().Run(ctx, filepath.Dir(plan.Path), plan.Rollback[0], plan.Rollback[1:]...)
			output = output + "\nrollback: " + rollbackOutput
			result.Output = output
			if rollbackErr == nil {
				status = "rolled_back"
				result.RolledBack = true
			} else {
				verifyErr = errors.Join(verifyErr, rollbackErr)
			}
		}
		u.record(ctx, plan, status, output, started)
		return result, verifyErr
	}

	u.record(ctx, plan, "applied", output, started)
	return Result{Version: after, Output: output}, nil
}

func (u *Updater) runner() Runner {
	if u.Runner != nil {
		return u.Runner
	}
	return ExecRunner{}
}

func (u *Updater) now() time.Time {
	if u.Now != nil {
		return u.Now()
	}
	return time.Now()
}

func (u *Updater) access() func(string) bool {
	if u.Access != nil {
		return u.Access
	}
	return writableDir
}

func (u *Updater) probe(ctx context.Context, plan Plan) (string, error) {
	if u.Probe != nil {
		return u.Probe(ctx, plan)
	}
	entry, ok := registry.ByName(plan.Name)
	if !ok {
		return "", fmt.Errorf("update: unknown tool %s", plan.Name)
	}
	timeout := u.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return detect.ProbeVersion(ctx, plan.Path, entry.VersionArgs, entry.VersionPattern, timeout)
}

func (u *Updater) record(ctx context.Context, plan Plan, status, output string, started time.Time) {
	if u.Store == nil {
		return
	}
	if len(output) > 8192 {
		output = output[:8192]
	}
	_, _ = u.Store.RecordUpdate(ctx, store.UpdateRecord{
		ToolID:      plan.ToolID,
		ToolName:    plan.Name,
		FromVersion: plan.From,
		ToVersion:   plan.To,
		Command:     strings.Join(plan.Command, " "),
		Status:      status,
		Output:      strings.TrimSpace(output),
		StartedAt:   started,
		FinishedAt:  u.now(),
	})
}

func commandFor(kind registry.UpdateKind, path, from, to string) ([]string, []string) {
	switch kind {
	case registry.KindNPM:
		return []string{path, "install", "-g", "npm@" + to},
			[]string{path, "install", "-g", "npm@" + from}
	case registry.KindUV:
		return []string{path, "self", "update", to},
			[]string{path, "self", "update", from}
	case registry.KindBun:
		return []string{path, "upgrade"}, nil
	case registry.KindDeno:
		return []string{path, "upgrade", "--version", to},
			[]string{path, "upgrade", "--version", from}
	case registry.KindComposer:
		return []string{path, "self-update", to},
			[]string{path, "self-update", from}
	case registry.KindRustup:
		return []string{path, "self", "update"}, nil
	default:
		return nil, nil
	}
}

func atLeast(version, target string) bool {
	return !check.IsNewer(target, version)
}

func writableDir(dir string) bool {
	return unix.Access(dir, unix.W_OK) == nil
}
