package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

func (c *Client) npmLatest(ctx context.Context, pkg string) (Result, error) {
	// The abbreviated packument is several MB smaller than the full document.
	// It keeps dist-tags but drops the publish-time map.
	data, err := c.getAccept(ctx, fmt.Sprintf("%s/%s", c.NPMBase, url.PathEscape(pkg)), "application/vnd.npm.install-v1+json")
	if err != nil {
		return Result{}, err
	}
	var doc struct {
		DistTags map[string]string `json:"dist-tags"`
		Time     map[string]string `json:"time"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Result{}, fmt.Errorf("parse npm %s: %w", pkg, err)
	}
	latest := doc.DistTags["latest"]
	if latest == "" {
		return Result{}, fmt.Errorf("npm %s: no latest dist-tag", pkg)
	}

	var published time.Time
	if raw := doc.Time[latest]; raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			published = parsed
		}
	}

	return Result{
		Version:   cleanVersion(latest),
		Published: published,
		URL:       fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", pkg, latest),
	}, nil
}
