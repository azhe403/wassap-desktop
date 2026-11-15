package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Asset is one file published on a GitHub Release.
type Asset struct {
	Name               string
	BrowserDownloadURL string
	Digest             string // e.g. "sha256:<hex>", as published by GitHub
}

// Release is the subset of a GitHub release this package needs.
type Release struct {
	TagName     string
	HTMLURL     string
	PublishedAt time.Time
	Prerelease  bool
	Assets      []Asset
}

// Client talks to the public GitHub REST API. It is unauthenticated and
// fail-soft: callers handle errors, never panic.
type Client struct {
	baseURL string
	repo    string
	hc      *http.Client
	ua      string
}

// NewClient returns a client for the public GitHub API.
func NewClient() *Client {
	return &Client{
		baseURL: "https://api.github.com",
		repo:    "azhe403/wassap-desktop",
		hc:      &http.Client{Timeout: 10 * time.Second},
		ua:      "wassap-desktop",
	}
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Assets      []ghAsset `json:"assets"`
}

type ghCompare struct {
	AheadBy int `json:"ahead_by"`
}

// LatestRelease fetches the newest non-prerelease release (GitHub's /releases/latest).
// A 404 means no stable release exists yet: it returns (nil, nil).
func (c *Client) LatestRelease(ctx context.Context) (*Release, error) {
	return c.fetchRelease(ctx, "/releases/latest", true)
}

// ReleaseByTag fetches the release attached to a tag (e.g. "nightly").
func (c *Client) ReleaseByTag(ctx context.Context, tag string) (*Release, error) {
	return c.fetchRelease(ctx, "/releases/tags/"+tag, false)
}

func (c *Client) fetchRelease(ctx context.Context, path string, allow404 bool) (*Release, error) {
	var gh ghRelease
	status, err := c.get(ctx, path, &gh)
	if err != nil {
		if allow404 && status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	out := &Release{
		TagName:     gh.TagName,
		HTMLURL:     gh.HTMLURL,
		PublishedAt: gh.PublishedAt,
		Prerelease:  gh.Prerelease,
	}
	for _, a := range gh.Assets {
		out.Assets = append(out.Assets, Asset{Name: a.Name, BrowserDownloadURL: a.BrowserDownloadURL, Digest: a.Digest})
	}
	return out, nil
}

// AheadBy reports how many commits head is ahead of base (GET /compare/base...head).
func (c *Client) AheadBy(ctx context.Context, base, head string) (int, error) {
	var cmp ghCompare
	_, err := c.get(ctx, "/compare/"+base+"..."+head, &cmp)
	if err != nil {
		return 0, err
	}
	return cmp.AheadBy, nil
}

// get performs one request and decodes the JSON body. HTTP statuses outside
// 2xx are errors carrying the status code.
func (c *Client) get(ctx context.Context, path string, out any) (int, error) {
	url := c.baseURL + "/repos/" + c.repo + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, fmt.Errorf("updater: github: %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return resp.StatusCode, fmt.Errorf("updater: github: unexpected status %d for %s", resp.StatusCode, path)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, fmt.Errorf("updater: github: decode %s: %w", path, err)
	}
	return resp.StatusCode, nil
}
