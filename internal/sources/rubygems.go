package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

func (c *Client) rubygemsLatest(ctx context.Context, gem string) (Result, error) {
	data, err := c.get(ctx, fmt.Sprintf("%s/api/v1/versions/%s/latest.json", c.RubyGemsBase, url.PathEscape(gem)))
	if err != nil {
		return Result{}, err
	}
	var doc struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Result{}, fmt.Errorf("parse rubygems %s: %w", gem, err)
	}
	if doc.Version == "" {
		return Result{}, fmt.Errorf("rubygems %s: no version", gem)
	}
	return Result{
		Version: cleanVersion(doc.Version),
		URL:     fmt.Sprintf("https://rubygems.org/gems/%s", gem),
	}, nil
}
