package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type dotnetRelease struct {
	ChannelVersion    string `json:"channel-version"`
	LatestSDK         string `json:"latest-sdk"`
	LatestReleaseDate string `json:"latest-release-date"`
	SupportPhase      string `json:"support-phase"`
	Security          bool   `json:"security"`
}

func (c *Client) dotnetLatest(ctx context.Context) (Result, error) {
	data, err := c.get(ctx, c.DotNetBase+"/dotnet/release-metadata/releases-index.json")
	if err != nil {
		return Result{}, err
	}
	var doc struct {
		Releases []dotnetRelease `json:"releases-index"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Result{}, fmt.Errorf("parse dotnet releases: %w", err)
	}

	for _, release := range doc.Releases {
		if release.LatestSDK == "" || strings.Contains(release.LatestSDK, "-") {
			continue
		}
		if strings.EqualFold(release.SupportPhase, "eol") {
			continue
		}
		var published time.Time
		if parsed, err := time.Parse("2006-01-02", release.LatestReleaseDate); err == nil {
			published = parsed
		}
		return Result{
			Version:   cleanVersion(release.LatestSDK),
			Published: published,
			URL:       "https://dotnet.microsoft.com/download",
			Security:  release.Security,
		}, nil
	}
	return Result{}, fmt.Errorf("dotnet: no supported SDK release")
}
