package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

func (c *Client) anacondaLatest(ctx context.Context, pkg string) (Result, error) {
	data, err := c.get(ctx, fmt.Sprintf("%s/package/%s/%s", c.AnacondaBase, "conda", pkg))
	if err != nil {
		return Result{}, err
	}
	var doc struct {
		Versions []string `json:"versions"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Result{}, fmt.Errorf("parse anaconda %s: %w", pkg, err)
	}

	var best *semver.Version
	bestRaw := ""
	for _, raw := range doc.Versions {
		lower := strings.ToLower(raw)
		if strings.Contains(lower, "rc") || strings.Contains(lower, "alpha") || strings.Contains(lower, "beta") || strings.Contains(lower, "dev") {
			continue
		}
		parsed, err := semver.NewVersion(raw)
		if err != nil {
			if bestRaw == "" {
				bestRaw = raw
			}
			continue
		}
		if best == nil || parsed.GreaterThan(best) {
			best = parsed
			bestRaw = raw
		}
	}
	if bestRaw == "" {
		return Result{}, fmt.Errorf("anaconda %s: no stable version", pkg)
	}
	return Result{
		Version: cleanVersion(bestRaw),
		URL:     fmt.Sprintf("https://anaconda.org/conda/%s", pkg),
	}, nil
}
