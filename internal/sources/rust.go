package sources

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var rustupVersion = regexp.MustCompile(`(?m)^version\s*=\s*['"]([^'"]+)['"]`)
var rustVersion = regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)

func (c *Client) rustupLatest(ctx context.Context) (Result, error) {
	data, err := c.get(ctx, c.RustupBase+"/rustup/release-stable.toml")
	if err != nil {
		return Result{}, err
	}
	match := rustupVersion.FindStringSubmatch(string(data))
	if len(match) < 2 {
		return Result{}, fmt.Errorf("rustup: version not found")
	}
	return Result{Version: cleanVersion(match[1]), URL: "https://rust-lang.github.io/rustup/"}, nil
}

func (c *Client) rustLatest(ctx context.Context) (Result, error) {
	data, err := c.get(ctx, c.RustBase+"/dist/channel-rust-stable.toml")
	if err != nil {
		return Result{}, err
	}
	body := string(data)
	idx := strings.Index(body, "[pkg.rust]")
	if idx < 0 {
		return Result{}, fmt.Errorf("rust: pkg.rust section not found")
	}
	match := rustVersion.FindStringSubmatch(body[idx:])
	if len(match) < 2 {
		return Result{}, fmt.Errorf("rust: version not found")
	}
	version := strings.Fields(match[1])[0]
	return Result{Version: cleanVersion(version), URL: "https://forge.rust-lang.org/"}, nil
}
