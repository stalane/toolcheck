package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type nodeRelease struct {
	Version  string `json:"version"`
	Date     string `json:"date"`
	LTS      any    `json:"lts"`
	Security bool   `json:"security"`
}

func (c *Client) nodeLatest(ctx context.Context, channel string) (Result, error) {
	data, err := c.get(ctx, c.NodeBase+"/index.json")
	if err != nil {
		return Result{}, err
	}
	var releases []nodeRelease
	if err := json.Unmarshal(data, &releases); err != nil {
		return Result{}, fmt.Errorf("parse node releases: %w", err)
	}

	wantLTS := strings.EqualFold(channel, "lts")
	for _, release := range releases {
		if strings.Contains(release.Version, "-") {
			continue
		}
		if wantLTS && !isLTS(release.LTS) {
			continue
		}
		var published time.Time
		if parsed, err := time.Parse("2006-01-02", release.Date); err == nil {
			published = parsed
		}
		version := cleanVersion(release.Version)
		return Result{
			Version:   version,
			Published: published,
			URL:       fmt.Sprintf("https://nodejs.org/dist/%s/", release.Version),
			Security:  release.Security,
		}, nil
	}
	return Result{}, fmt.Errorf("%w: node channel %q", ErrNoRelease, channel)
}

func isLTS(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed != "" && typed != "false"
	case bool:
		return typed
	default:
		return false
	}
}
