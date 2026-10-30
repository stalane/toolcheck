package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) githubLatest(ctx context.Context, repo string) (Result, error) {
	requestURL := fmt.Sprintf("%s/%s/releases/latest", c.GitHubBase, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	client := *c.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return Result{}, fmt.Errorf("%w: github %s (%s)", ErrNoRelease, repo, resp.Status)
	}
	location := resp.Header.Get("Location")
	if location == "" {
		return Result{}, fmt.Errorf("github %s: empty redirect", repo)
	}

	base, _ := url.Parse(requestURL)
	ref, err := url.Parse(location)
	if err != nil {
		return Result{}, fmt.Errorf("github %s: bad redirect %q", repo, location)
	}
	target := base.ResolveReference(ref)
	path := target.Path
	idx := strings.Index(path, "/tag/")
	if idx < 0 {
		return Result{}, fmt.Errorf("github %s: redirect is not a tag: %s", repo, target)
	}
	tag, err := url.PathUnescape(path[idx+5:])
	if err != nil {
		tag = path[idx+5:]
	}
	if tag == "" {
		return Result{}, fmt.Errorf("github %s: empty tag", repo)
	}

	return Result{
		Version: cleanVersion(tag),
		URL:     fmt.Sprintf("%s/%s", c.GitHubBase, repo),
	}, nil
}
