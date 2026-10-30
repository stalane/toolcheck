package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type CheckConfig struct {
	Interval Duration `yaml:"interval"`
	Timeout  Duration `yaml:"timeout"`
	CacheTTL Duration `yaml:"cache_ttl"`
}

type ToolsConfig struct {
	Include       []string `yaml:"include"`
	Exclude       []string `yaml:"exclude"`
	HideOSManaged bool     `yaml:"hide_os_managed"`
	ShowInactive  bool     `yaml:"show_inactive"`
	NodeChannel   string   `yaml:"node_channel"`
}

type UpdateConfig struct {
	Enabled       bool     `yaml:"enabled"`
	MinReleaseAge Duration `yaml:"min_release_age"`
}

type NotifyConfig struct {
	Desktop bool `yaml:"desktop"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type Config struct {
	Check  CheckConfig  `yaml:"check"`
	Tools  ToolsConfig  `yaml:"tools"`
	Update UpdateConfig `yaml:"update"`
	Notify NotifyConfig `yaml:"notify"`
	Server ServerConfig `yaml:"server"`
}

func Default() Config {
	return Config{
		Check: CheckConfig{
			Interval: Duration(24 * time.Hour),
			Timeout:  Duration(15 * time.Second),
			CacheTTL: Duration(24 * time.Hour),
		},
		Tools: ToolsConfig{
			NodeChannel: "lts",
		},
		Update: UpdateConfig{
			Enabled:       true,
			MinReleaseAge: Duration(72 * time.Hour),
		},
		Notify: NotifyConfig{Desktop: true},
		Server: ServerConfig{Addr: "127.0.0.1:8788"},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	c := Default()
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return c, nil
}

func (c Config) Validate() error {
	var problems []string

	if c.Check.Interval.Duration() <= 0 {
		problems = append(problems, "check.interval must be greater than zero")
	}
	if c.Check.Timeout.Duration() <= 0 {
		problems = append(problems, "check.timeout must be greater than zero")
	}
	if c.Check.CacheTTL.Duration() < 0 {
		problems = append(problems, "check.cache_ttl must not be negative")
	}
	if c.Update.MinReleaseAge.Duration() < 0 {
		problems = append(problems, "update.min_release_age must not be negative")
	}
	switch strings.ToLower(strings.TrimSpace(c.Tools.NodeChannel)) {
	case "", "lts", "current":
	default:
		problems = append(problems, fmt.Sprintf("tools.node_channel must be lts or current, got %q", c.Tools.NodeChannel))
	}
	if err := validateLoopbackAddr(c.Server.Addr); err != nil {
		problems = append(problems, "server.addr: "+err.Error())
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (c Config) NodeChannel() string {
	channel := strings.ToLower(strings.TrimSpace(c.Tools.NodeChannel))
	if channel == "" {
		return "lts"
	}
	return channel
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(base, "toolcheck"), nil
}

func DefaultPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func DataDir() (string, error) {
	if dir := os.Getenv("TOOLCHECK_DATA_DIR"); dir != "" {
		return dir, nil
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "toolcheck"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "toolcheck"), nil
}

func Complete(path string) (string, error) {
	if path == "" {
		return DefaultPath()
	}
	return path, nil
}

func ValidateLoopbackAddr(addr string) error {
	return validateLoopbackAddr(addr)
}

func validateLoopbackAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("must be host:port, %w", err)
	}
	if port == "" {
		return errors.New("port must not be empty")
	}

	switch host {
	case "127.0.0.1", "::1", "localhost":
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("must bind a loopback address (127.0.0.1, ::1 or localhost), got %q", host)
}

func validateBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return errors.New("host must not be empty")
	}
	return nil
}
