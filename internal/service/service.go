package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const unitName = "toolcheck.service"

func Content(binaryPath string) string {
	return fmt.Sprintf(`[Unit]
Description=toolcheck package manager freshness
After=network-online.target

[Service]
ExecStart="%s" serve
Restart=on-failure
RestartSec=10

[Install]
WantedBy=default.target
`, binaryPath)
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(base, "systemd", "user"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, unitName), nil
}

func Install(binaryPath string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create systemd user dir %s: %w", dir, err)
	}

	path := filepath.Join(dir, unitName)
	tmp, err := os.CreateTemp(dir, ".toolcheck-*")
	if err != nil {
		return "", fmt.Errorf("create temp unit: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(Content(binaryPath)); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write unit: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close unit: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return "", fmt.Errorf("chmod unit: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", fmt.Errorf("install unit: %w", err)
	}

	if err := run("systemctl", "--user", "daemon-reload"); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := run("systemctl", "--user", "enable", "--now", unitName); err != nil {
		return "", err
	}
	return path, nil
}

func Uninstall() error {
	_ = run("systemctl", "--user", "disable", "--now", unitName)
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit %s: %w", path, err)
	}
	_ = run("systemctl", "--user", "daemon-reload")
	return nil
}

func run(name string, args ...string) error {
	output, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
