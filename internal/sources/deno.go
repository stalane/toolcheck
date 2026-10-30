package sources

import (
	"context"
	"fmt"
	"strings"
)

func (c *Client) denoLatest(ctx context.Context) (Result, error) {
	data, err := c.get(ctx, c.DenoBase+"/release-latest.txt")
	if err != nil {
		return Result{}, err
	}
	version := cleanVersion(strings.TrimSpace(string(data)))
	if version == "" {
		return Result{}, fmt.Errorf("deno: empty latest version")
	}
	return Result{Version: version, URL: "https://deno.com/blog"}, nil
}
