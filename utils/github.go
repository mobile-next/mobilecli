package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type GitHubRelease struct {
	Assets []struct {
		BrowserDownloadURL string `json:"browser_download_url"`
		Name               string `json:"name"`
	} `json:"assets"`
}

// GetLatestReleaseDownloadURL fetches the latest release from a GitHub repository
// and returns the browser download URL of the first asset
func GetLatestReleaseDownloadURL(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to decode release JSON: %v", err)
	}

	if len(release.Assets) == 0 {
		return "", fmt.Errorf("no assets found in latest release")
	}

	return release.Assets[0].BrowserDownloadURL, nil
}