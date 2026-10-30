package sources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrNoSource  = errors.New("no upstream source")
	ErrNoRelease = errors.New("no upstream release")
)

type Result struct {
	Version   string
	Published time.Time
	URL       string
	Security  bool
}

type Client struct {
	HTTP         *http.Client
	UserAgent    string
	MaxBytes     int64
	NPMBase      string
	PyPIBase     string
	NodeBase     string
	GoBase       string
	DenoBase     string
	ComposerBase string
	RustupBase   string
	RustBase     string
	GitHubBase   string
	AnacondaBase string
	RubyGemsBase string
	DotNetBase   string
}

func New(userAgent string) *Client {
	return &Client{
		HTTP:         &http.Client{Timeout: 15 * time.Second},
		UserAgent:    userAgent,
		MaxBytes:     8 << 20,
		NPMBase:      "https://registry.npmjs.org",
		PyPIBase:     "https://pypi.org",
		NodeBase:     "https://nodejs.org/dist",
		GoBase:       "https://go.dev/dl/",
		DenoBase:     "https://dl.deno.land",
		ComposerBase: "https://getcomposer.org",
		RustupBase:   "https://static.rust-lang.org",
		RustBase:     "https://static.rust-lang.org",
		GitHubBase:   "https://github.com",
		AnacondaBase: "https://api.anaconda.org",
		RubyGemsBase: "https://rubygems.org",
		DotNetBase:   "https://dotnetcli.blob.core.windows.net",
	}
}

func (c *Client) Fetch(ctx context.Context, spec string) (Result, error) {
	switch {
	case spec == "" || spec == "none":
		return Result{}, ErrNoSource
	case strings.HasPrefix(spec, "npm:"):
		return c.npmLatest(ctx, strings.TrimPrefix(spec, "npm:"))
	case strings.HasPrefix(spec, "pypi:"):
		return c.pypiLatest(ctx, strings.TrimPrefix(spec, "pypi:"))
	case spec == "node" || strings.HasPrefix(spec, "node:"):
		channel := "lts"
		if parts := strings.SplitN(spec, ":", 2); len(parts) == 2 {
			channel = parts[1]
		}
		return c.nodeLatest(ctx, channel)
	case spec == "go":
		return c.goLatest(ctx)
	case spec == "deno":
		return c.denoLatest(ctx)
	case spec == "composer":
		return c.composerLatest(ctx)
	case spec == "rustup":
		return c.rustupLatest(ctx)
	case spec == "rust":
		return c.rustLatest(ctx)
	case strings.HasPrefix(spec, "github:"):
		return c.githubLatest(ctx, strings.TrimPrefix(spec, "github:"))
	case strings.HasPrefix(spec, "anaconda:"):
		return c.anacondaLatest(ctx, strings.TrimPrefix(spec, "anaconda:"))
	case strings.HasPrefix(spec, "rubygems:"):
		return c.rubygemsLatest(ctx, strings.TrimPrefix(spec, "rubygems:"))
	case spec == "dotnet":
		return c.dotnetLatest(ctx)
	default:
		return Result{}, fmt.Errorf("unknown source %q", spec)
	}
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	return c.getAccept(ctx, url, "application/json, text/plain;q=0.9, */*;q=0.1")
}

func (c *Client) getAccept(ctx context.Context, url, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", accept)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	max := c.MaxBytes
	if max <= 0 {
		max = 8 << 20
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("GET %s: response over %d bytes", url, max)
	}
	return data, nil
}

func cleanVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	return version
}
