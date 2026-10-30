package detect

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/stalane/toolcheck/internal/registry"
)

type Tool struct {
	Name       string
	Family     string
	Path       string
	RealPath   string
	Version    string
	Origin     string
	ManagedBy  string
	Risk       string
	Source     string
	Hint       string
	Active     bool
	Writable   bool
	Updateable bool
	LatestOnly bool
}

type Options struct {
	Timeout     time.Duration
	LookPathAll func(string) []string
	RunVersion  func(context.Context, string, ...string) (string, error)
	DpkgOwner   func(string) bool
	Writable    func(string) bool
	HomeDir     func() (string, error)
	Getenv      func(string) string
}

type Detector struct {
	opts Options
}

func New(opts Options) *Detector {
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	if opts.LookPathAll == nil {
		opts.LookPathAll = lookPathAll
	}
	if opts.RunVersion == nil {
		opts.RunVersion = runVersion
	}
	if opts.DpkgOwner == nil {
		opts.DpkgOwner = dpkgOwner
	}
	if opts.Writable == nil {
		opts.Writable = writableDir
	}
	if opts.HomeDir == nil {
		opts.HomeDir = os.UserHomeDir
	}
	if opts.Getenv == nil {
		opts.Getenv = os.Getenv
	}
	return &Detector{opts: opts}
}

func (d *Detector) Detect(ctx context.Context, entries []registry.Tool) []Tool {
	var tools []Tool
	for _, entry := range entries {
		if entry.Synthetic != nil {
			if tool, ok := d.detectSynthetic(entry); ok {
				tools = append(tools, tool)
			}
			continue
		}

		seen := make(map[string]bool)
		active := true
		for _, command := range entry.Commands {
			for _, path := range d.opts.LookPathAll(command) {
				realPath, err := filepath.EvalSymlinks(path)
				if err != nil || realPath == "" {
					realPath = path
				}
				if seen[realPath] {
					continue
				}
				seen[realPath] = true

				version := d.version(ctx, path, entry)
				tool := d.build(entry, path, realPath, version, active)
				tools = append(tools, tool)
				active = false
			}
		}
	}
	return tools
}

func (d *Detector) build(entry registry.Tool, path, realPath, version string, active bool) Tool {
	origin, managedBy := d.classify(realPath)
	writable := d.opts.Writable(filepath.Dir(realPath))
	updateable := entry.Update != nil && managedBy == "self" && writable

	risk := "alert"
	if updateable {
		risk = "safe"
	}
	switch managedBy {
	case "apt", "brew", "nix", "snap", "flatpak":
		risk = "system"
	}

	tool := Tool{
		Name:       entry.Name,
		Family:     entry.Family,
		Path:       path,
		RealPath:   realPath,
		Version:    version,
		Origin:     origin,
		ManagedBy:  managedBy,
		Risk:       risk,
		Source:     entry.Source,
		Hint:       entry.Hint,
		Active:     active,
		Writable:   writable,
		Updateable: updateable,
	}
	if entry.Update != nil {
		tool.LatestOnly = entry.Update.LatestOnly
	}
	if tool.Version == "" {
		tool.Version = "unknown"
	}
	return tool
}

func (d *Detector) detectSynthetic(entry registry.Tool) (Tool, bool) {
	dir := d.syntheticDir(entry.Synthetic)
	if dir == "" {
		return Tool{}, false
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return Tool{}, false
	}

	version := "unknown"
	if entry.Synthetic.VersionFile != "" {
		if parsed := readVersionFile(filepath.Join(dir, entry.Synthetic.VersionFile), entry.Synthetic.VersionJSON); parsed != "" {
			version = parsed
		}
	}

	return Tool{
		Name:       entry.Name,
		Family:     entry.Family,
		Path:       dir,
		RealPath:   dir,
		Version:    version,
		Origin:     entry.Name,
		ManagedBy:  "version-manager",
		Risk:       "alert",
		Source:     entry.Source,
		Hint:       entry.Hint,
		Active:     true,
		Writable:   false,
		Updateable: false,
	}, true
}

func (d *Detector) syntheticDir(spec *registry.Synthetic) string {
	if spec.DirEnv != "" {
		if dir := d.opts.Getenv(spec.DirEnv); dir != "" {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir
			}
		}
	}
	home, _ := d.opts.HomeDir()
	for _, hint := range spec.DirHints {
		if strings.HasPrefix(hint, "~/") {
			if home == "" {
				continue
			}
			hint = filepath.Join(home, hint[2:])
		}
		if info, err := os.Stat(hint); err == nil && info.IsDir() {
			return hint
		}
	}
	return ""
}

func (d *Detector) version(ctx context.Context, path string, entry registry.Tool) string {
	args := entry.VersionArgs
	if len(args) == 0 {
		args = []string{"--version"}
	}
	probeCtx, cancel := context.WithTimeout(ctx, d.opts.Timeout)
	defer cancel()
	output, _ := d.opts.RunVersion(probeCtx, path, args...)
	return parseVersion(output, entry.VersionPattern)
}

func (d *Detector) classify(realPath string) (string, string) {
	home, _ := d.opts.HomeDir()
	clean := filepath.Clean(realPath)

	if strings.HasPrefix(clean, "/nix/store/") {
		return "nix", "nix"
	}
	if strings.HasPrefix(clean, "/snap/") {
		return "snap", "snap"
	}
	if strings.HasPrefix(clean, "/var/lib/flatpak/") || strings.Contains(clean, "/.local/share/flatpak/") {
		return "flatpak", "flatpak"
	}
	if prefix := d.opts.Getenv("HOMEBREW_PREFIX"); prefix != "" && under(clean, prefix) {
		return "homebrew", "brew"
	}
	for _, prefix := range []string{"/home/linuxbrew/.linuxbrew", "/opt/homebrew"} {
		if under(clean, prefix) {
			return "homebrew", "brew"
		}
	}
	if strings.Contains(clean, "/corepack/") {
		return "corepack", "corepack"
	}

	for _, hint := range managerHints(home, d.opts.Getenv) {
		for _, dir := range hint.dirs {
			if dir != "" && under(clean, dir) {
				return hint.origin, "version-manager"
			}
		}
	}

	if dir := firstNonEmpty(d.opts.Getenv("PIPX_HOME"), filepath.Join(home, ".local/share/pipx")); dir != "" && under(clean, dir) {
		return "pipx", "pipx"
	}
	if dir := firstNonEmpty(d.opts.Getenv("UV_TOOL_DIR"), filepath.Join(home, ".local/share/uv/tools")); dir != "" && under(clean, dir) {
		return "uv-tool", "uv"
	}
	if dir := firstNonEmpty(d.opts.Getenv("BUN_INSTALL"), filepath.Join(home, ".bun")); dir != "" && under(clean, dir) {
		return "bun", "self"
	}
	if dir := filepath.Join(home, ".npm-global"); under(clean, dir) {
		return "npm-global", "self"
	}
	if dir := filepath.Join(home, ".local/lib/node_modules"); under(clean, dir) {
		return "npm-global", "self"
	}
	if dir := firstNonEmpty(d.opts.Getenv("NPM_CONFIG_PREFIX"), ""); dir != "" && under(clean, dir) {
		return "npm-global", "self"
	}
	if dir := filepath.Join(home, ".cargo/bin"); under(clean, dir) {
		return "cargo-home", "self"
	}
	if dir := filepath.Join(home, ".deno"); under(clean, dir) {
		return "deno", "self"
	}

	if d.opts.DpkgOwner(clean) {
		return "apt", "apt"
	}

	if home != "" && under(clean, home) {
		if d.opts.Writable(filepath.Dir(clean)) {
			return "self", "self"
		}
		return "home-readonly", "manual"
	}
	for _, prefix := range []string{"/usr/local", "/opt"} {
		if under(clean, prefix) {
			return "manual", "manual"
		}
	}
	return "unknown", "unknown"
}

type managerHint struct {
	origin string
	dirs   []string
}

func managerHints(home string, getenv func(string) string) []managerHint {
	expand := func(path string) string {
		if strings.HasPrefix(path, "~/") && home != "" {
			return filepath.Join(home, path[2:])
		}
		return path
	}
	hints := []managerHint{
		{origin: "nvm", dirs: []string{getenv("NVM_DIR"), expand("~/.nvm")}},
		{origin: "fnm", dirs: []string{getenv("FNM_DIR"), expand("~/.local/share/fnm"), expand("~/.fnm")}},
		{origin: "volta", dirs: []string{getenv("VOLTA_HOME"), expand("~/.volta")}},
		{origin: "asdf", dirs: []string{getenv("ASDF_DIR"), getenv("ASDF_DATA_DIR"), expand("~/.asdf")}},
		{origin: "mise", dirs: []string{getenv("MISE_DATA_DIR"), expand("~/.local/share/mise"), expand("~/.mise")}},
		{origin: "pyenv", dirs: []string{getenv("PYENV_ROOT"), expand("~/.pyenv")}},
		{origin: "rbenv", dirs: []string{getenv("RBENV_ROOT"), expand("~/.rbenv")}},
		{origin: "rvm", dirs: []string{getenv("RVM_DIR"), expand("~/.rvm")}},
		{origin: "sdkman", dirs: []string{getenv("SDKMAN_DIR"), expand("~/.sdkman")}},
		{origin: "gvm", dirs: []string{getenv("GVM_ROOT"), expand("~/.gvm")}},
		{origin: "g", dirs: []string{expand("~/.g")}},
	}
	return hints
}

func under(path, dir string) bool {
	if dir == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

var genericVersion = regexp.MustCompile(`(?i)(?:version[:\s]+)?v?(\d+\.\d+(?:\.\d+)?(?:-[0-9A-Za-z.-]+)?)`)

func parseVersion(output, pattern string) string {
	if pattern != "" {
		if re, err := regexp.Compile(pattern); err == nil {
			if match := re.FindStringSubmatch(output); len(match) > 1 {
				return cleanVersion(match[1])
			} else if len(match) == 1 {
				return cleanVersion(match[0])
			}
		}
	}
	if match := genericVersion.FindStringSubmatch(output); len(match) > 1 {
		return cleanVersion(match[1])
	}
	return ""
}

// ParseVersion extracts a version from command output using the registry pattern,
// falling back to the first numeric version-like token.
func ParseVersion(output, pattern string) string {
	return parseVersion(output, pattern)
}

// ProbeVersion runs a tool's version command and parses the result.
func ProbeVersion(ctx context.Context, path string, args []string, pattern string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, path, args...)
	output, err := cmd.CombinedOutput()
	return parseVersion(string(output), pattern), err
}

func cleanVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "go")
	return version
}

func readVersionFile(path string, asJSON bool) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if asJSON {
		var doc struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &doc); err == nil && doc.Version != "" {
			return cleanVersion(doc.Version)
		}
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
	return cleanVersion(line)
}

func lookPathAll(name string) []string {
	var paths []string
	seen := make(map[string]bool)
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		real, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			real = candidate
		}
		if seen[real] {
			continue
		}
		seen[real] = true
		paths = append(paths, candidate)
	}
	return paths
}

func runVersion(ctx context.Context, path string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func dpkgOwner(path string) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	output, err := exec.Command("dpkg-query", "-S", path).CombinedOutput()
	return err == nil && len(strings.TrimSpace(string(output))) > 0
}

func writableDir(dir string) bool {
	return unix.Access(dir, unix.W_OK) == nil
}
