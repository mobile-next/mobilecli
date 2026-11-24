package utils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDownloadFile_Success(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "robots.txt")

	err := DownloadFile("https://github.com/robots.txt", tmpFile)
	assert.NoError(t, err, "Download should succeed")
	assert.FileExists(t, tmpFile, "Downloaded file should exist")

	content, err := os.ReadFile(tmpFile)
	assert.NoError(t, err, "Should be able to read downloaded file")
	assert.NotEmpty(t, content, "Downloaded file should not be empty")

	contentStr := string(content)
	assert.Contains(t, contentStr, "User-agent", "robots.txt should contain User-agent directive")

	info, err := os.Stat(tmpFile)
	assert.NoError(t, err, "Should be able to stat downloaded file")
	assert.Greater(t, info.Size(), int64(0), "Downloaded file should have non-zero size")
}

func TestDownloadFile_HTTPError(t *testing.T) {
	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer func() { server.Close() }()

	tmpFile := filepath.Join(t.TempDir(), "download_test.txt")
	err := DownloadFile(server.URL, tmpFile)

	assert.Error(t, err, "Should return error for 404 response")
	assert.Contains(t, err.Error(), "download returned status 404", "Error should mention status code")

	assert.NoFileExists(t, tmpFile, "File should not exist after failed download")
}
