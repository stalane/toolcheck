package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (c *Client) goLatest(ctx context.Context) (Result, error) {
	data, err := c.get(ctx, strings.TrimRight(c.GoBase, "/")+"/?mode=json")
	if err != nil {
		return Result{}, err
	}
	var releases []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &releases); err != nil {
		return Result{}, fmt.Errorf("parse go releases: %w", err)
	}
	if len(releases) == 0 || releases[0].Version == "" {
		return Result{}, fmt.Errorf("go: no stable release")
	}
	return Result{
		Version: strings.TrimPrefix(releases[0].Version, "go"),
		URL:     "https://go.dev/dl/",
	}, nil
}
