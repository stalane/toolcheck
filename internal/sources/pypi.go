package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

func (c *Client) pypiLatest(ctx context.Context, pkg string) (Result, error) {
	data, err := c.get(ctx, fmt.Sprintf("%s/pypi/%s/json", c.PyPIBase, url.PathEscape(pkg)))
	if err != nil {
		return Result{}, err
	}
	var doc struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
		Releases map[string][]struct {
			UploadTimeISO string `json:"upload_time_iso_8601"`
		} `json:"releases"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Result{}, fmt.Errorf("parse pypi %s: %w", pkg, err)
	}
	if doc.Info.Version == "" {
		return Result{}, fmt.Errorf("pypi %s: no version", pkg)
	}

	var published time.Time
	if files := doc.Releases[doc.Info.Version]; len(files) > 0 && files[0].UploadTimeISO != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, files[0].UploadTimeISO); err == nil {
			published = parsed
		}
	}

	return Result{
		Version:   cleanVersion(doc.Info.Version),
		Published: published,
		URL:       fmt.Sprintf("https://pypi.org/project/%s/%s/", pkg, doc.Info.Version),
	}, nil
}
