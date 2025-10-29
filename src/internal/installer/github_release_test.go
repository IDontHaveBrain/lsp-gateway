package installer

import (
	"testing"
)

func TestNewGitHubReleaseFetcher(t *testing.T) {
	fetcher := NewGitHubReleaseFetcher("owner", "repo")

	if fetcher == nil {
		t.Fatal("Expected non-nil fetcher")
	}

	if fetcher.owner != "owner" {
		t.Errorf("Expected owner 'owner', got '%s'", fetcher.owner)
	}

	if fetcher.repo != "repo" {
		t.Errorf("Expected repo 'repo', got '%s'", fetcher.repo)
	}

	if fetcher.httpClient == nil {
		t.Error("Expected non-nil HTTP client")
	}
}

func TestFindAssetURL(t *testing.T) {
	release := &GitHubRelease{
		TagName: "v1.0.0",
		Assets: []GitHubAsset{
			{Name: "app-linux-x64.tar.gz", BrowserDownloadURL: "https://example.com/linux.tar.gz"},
			{Name: "app-windows-x64.zip", BrowserDownloadURL: "https://example.com/windows.zip"},
			{Name: "app-darwin-arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin.tar.gz"},
		},
	}

	tests := []struct {
		name        string
		pattern     func(string) bool
		expectURL   string
		expectError bool
	}{
		{
			name:        "Linux asset",
			pattern:     func(name string) bool { return name == "app-linux-x64.tar.gz" },
			expectURL:   "https://example.com/linux.tar.gz",
			expectError: false,
		},
		{
			name:        "Windows asset",
			pattern:     func(name string) bool { return name == "app-windows-x64.zip" },
			expectURL:   "https://example.com/windows.zip",
			expectError: false,
		},
		{
			name:        "No match",
			pattern:     func(name string) bool { return name == "nonexistent.tar.gz" },
			expectURL:   "",
			expectError: true,
		},
		{
			name:        "Pattern matching linux",
			pattern:     func(name string) bool { return len(name) > 15 && name[4] == 'l' },
			expectURL:   "https://example.com/linux.tar.gz",
			expectError: false,
		},
	}

	fetcher := NewGitHubReleaseFetcher("owner", "repo")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := fetcher.FindAssetURL(release, tt.pattern)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if url != tt.expectURL {
					t.Errorf("Expected URL '%s', got '%s'", tt.expectURL, url)
				}
			}
		})
	}
}
