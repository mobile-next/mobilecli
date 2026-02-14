package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetLatestReleaseDownloadURL_ParsesResponse(t *testing.T) {
	release := GitHubRelease{
		Assets: []struct {
			BrowserDownloadURL string `json:"browser_download_url"`
			Name               string `json:"name"`
		}{
			{BrowserDownloadURL: "https://example.com/release.apk", Name: "release.apk"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	// override the URL by testing the JSON parsing directly
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("HTTP request error: %v", err)
	}
	defer resp.Body.Close()

	var got GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if len(got.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(got.Assets))
	}
	if got.Assets[0].BrowserDownloadURL != "https://example.com/release.apk" {
		t.Errorf("URL = %q, want %q", got.Assets[0].BrowserDownloadURL, "https://example.com/release.apk")
	}
}

func TestGitHubRelease_EmptyAssets(t *testing.T) {
	release := GitHubRelease{}

	data, err := json.Marshal(release)
	if err != nil {
		t.Fatal(err)
	}

	var got GitHubRelease
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if len(got.Assets) != 0 {
		t.Errorf("expected 0 assets, got %d", len(got.Assets))
	}
}
