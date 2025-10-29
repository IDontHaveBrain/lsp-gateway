package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type GitHubReleaseFetcher struct {
	owner      string
	repo       string
	httpClient *http.Client
}

func NewGitHubReleaseFetcher(owner, repo string) *GitHubReleaseFetcher {
	return &GitHubReleaseFetcher{
		owner: owner,
		repo:  repo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (g *GitHubReleaseFetcher) FetchLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", g.owner, g.repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release JSON: %w", err)
	}

	return &release, nil
}

func (g *GitHubReleaseFetcher) FindAssetURL(release *GitHubRelease, assetNamePattern func(string) bool) (string, error) {
	for _, asset := range release.Assets {
		if assetNamePattern(asset.Name) {
			return asset.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("no matching asset found in release %s", release.TagName)
}

func (g *GitHubReleaseFetcher) DownloadAsset(ctx context.Context, base *BaseInstaller, assetURL, destPath string) error {
	return base.DownloadFile(ctx, assetURL, destPath)
}
