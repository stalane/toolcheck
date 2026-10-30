package sources

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) composerLatest(ctx context.Context) (Result, error) {
	data, err := c.get(ctx, c.ComposerBase+"/versions")
	if err != nil {
		return Result{}, err
	}
	var doc struct {
		Stable []struct {
			Version string `json:"version"`
		} `json:"stable"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Result{}, fmt.Errorf("parse composer versions: %w", err)
	}
	if len(doc.Stable) == 0 || doc.Stable[0].Version == "" {
		return Result{}, fmt.Errorf("composer: no stable version")
	}
	return Result{
		Version: cleanVersion(doc.Stable[0].Version),
		URL:     "https://getcomposer.org/download/",
	}, nil
}
