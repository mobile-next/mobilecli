package utils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadFile_Success(t *testing.T) {
	// Test successful download using real GitHub robots.txt
	tmpFile := filepath.Join(t.TempDir(), "robots.txt")
	
	err := DownloadFile("https://github.com/robots.txt", tmpFile)
	assert.NoError(t, err, "Download should succeed")
	
	// Verify file was created
	assert.FileExists(t, tmpFile, "Downloaded file should exist")
	
	// Verify file has content
	content, err := os.ReadFile(tmpFile)
	assert.NoError(t, err, "Should be able to read downloaded file")
	assert.NotEmpty(t, content, "Downloaded file should not be empty")
	
	// Verify it looks like a robots.txt file
	contentStr := string(content)
	assert.Contains(t, contentStr, "User-agent", "robots.txt should contain User-agent directive")
	
	// Verify file info
	info, err := os.Stat(tmpFile)
	assert.NoError(t, err, "Should be able to stat downloaded file")
	assert.Greater(t, info.Size(), int64(0), "Downloaded file should have non-zero size")
}

func TestDownloadFile_HTTPError(t *testing.T) {
	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()
	
	tmpFile := filepath.Join(t.TempDir(), "download_test.txt")
	err := DownloadFile(server.URL, tmpFile)
	
	assert.Error(t, err, "Should return error for 404 response")
	assert.Contains(t, err.Error(), "download returned status 404", "Error should mention status code")
	
	// File should not be created on error
	assert.NoFileExists(t, tmpFile, "File should not exist after failed download")
}

func TestDownloadFile_ServerError(t *testing.T) {
	// Create test server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()
	
	tmpFile := filepath.Join(t.TempDir(), "download_test.txt")
	err := DownloadFile(server.URL, tmpFile)
	
	assert.Error(t, err, "Should return error for 500 response")
	assert.Contains(t, err.Error(), "download returned status 500", "Error should mention status code")
}

func TestDownloadFile_InvalidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"Invalid scheme", "invalid://url"},
		{"Malformed URL", "not-a-url"},
		{"Empty URL", ""},
		{"Non-existent domain", "http://this-domain-does-not-exist-12345.com/file.txt"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "download_test.txt")
			err := DownloadFile(tt.url, tmpFile)
			
			assert.Error(t, err, "Should return error for invalid URL: %s", tt.url)
			assert.Contains(t, err.Error(), "failed to download file", "Error should mention download failure")
			
			// File should not be created on error
			assert.NoFileExists(t, tmpFile, "File should not exist after failed download")
		})
	}
}

func TestDownloadFile_FileCreationError(t *testing.T) {
	// Create test server with successful response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test content"))
	}))
	defer server.Close()
	
	tests := []struct {
		name     string
		filePath string
	}{
		{"Non-existent directory", "/non/existent/directory/file.txt"},
		{"Invalid filename", "/dev/null/invalid"}, // Can't create file inside /dev/null
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DownloadFile(server.URL, tt.filePath)
			
			assert.Error(t, err, "Should return error for invalid file path: %s", tt.filePath)
			assert.Contains(t, err.Error(), "failed to create file", "Error should mention file creation failure")
		})
	}
}

func TestDownloadFile_LargeFile(t *testing.T) {
	// Create test server with large content
	largeContent := strings.Repeat("A", 1024*1024) // 1MB
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeContent))
	}))
	defer server.Close()
	
	tmpFile := filepath.Join(t.TempDir(), "large_file.txt")
	err := DownloadFile(server.URL, tmpFile)
	
	assert.NoError(t, err, "Should successfully download large file")
	
	// Verify file size
	info, err := os.Stat(tmpFile)
	assert.NoError(t, err, "Should be able to stat large file")
	assert.Equal(t, int64(len(largeContent)), info.Size(), "Downloaded file should have correct size")
	
	// Verify content (sample check)
	content, err := os.ReadFile(tmpFile)
	assert.NoError(t, err, "Should be able to read large file")
	assert.Equal(t, largeContent, string(content), "Downloaded content should match")
}

func TestDownloadFile_EmptyFile(t *testing.T) {
	// Create test server with empty response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// No content written
	}))
	defer server.Close()
	
	tmpFile := filepath.Join(t.TempDir(), "empty_file.txt")
	err := DownloadFile(server.URL, tmpFile)
	
	assert.NoError(t, err, "Should successfully download empty file")
	
	// Verify file exists but is empty
	assert.FileExists(t, tmpFile, "Empty file should still be created")
	
	info, err := os.Stat(tmpFile)
	assert.NoError(t, err, "Should be able to stat empty file")
	assert.Equal(t, int64(0), info.Size(), "Empty file should have zero size")
}

func TestDownloadFile_OverwriteExisting(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("new content"))
	}))
	defer server.Close()
	
	tmpFile := filepath.Join(t.TempDir(), "overwrite_test.txt")
	
	// Create existing file with different content
	err := os.WriteFile(tmpFile, []byte("old content"), 0644)
	require.NoError(t, err, "Should be able to create initial file")
	
	// Download should overwrite existing file
	err = DownloadFile(server.URL, tmpFile)
	assert.NoError(t, err, "Should successfully overwrite existing file")
	
	// Verify new content
	content, err := os.ReadFile(tmpFile)
	assert.NoError(t, err, "Should be able to read overwritten file")
	assert.Equal(t, "new content", string(content), "File should contain new content")
}

func TestDownloadFile_ContentTypes(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		content     string
	}{
		{"Plain text", "text/plain", "Hello World"},
		{"JSON", "application/json", `{"key": "value"}`},
		{"HTML", "text/html", "<html><body>Test</body></html>"},
		{"Binary", "application/octet-stream", "\x00\x01\x02\x03"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server with specific content type
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.content))
			}))
			defer server.Close()
			
			tmpFile := filepath.Join(t.TempDir(), "content_type_test.txt")
			err := DownloadFile(server.URL, tmpFile)
			
			assert.NoError(t, err, "Should successfully download %s content", tt.name)
			
			// Verify content
			content, err := os.ReadFile(tmpFile)
			assert.NoError(t, err, "Should be able to read downloaded %s file", tt.name)
			assert.Equal(t, tt.content, string(content), "Downloaded %s content should match", tt.name)
		})
	}
}

func TestDownloadFile_HTTPSRedirect(t *testing.T) {
	// Test with a URL that typically redirects (http to https)
	tmpFile := filepath.Join(t.TempDir(), "redirect_test.txt")
	
	// Use http://github.com which should redirect to https
	err := DownloadFile("http://github.com/robots.txt", tmpFile)
	
	// Should succeed even with redirect
	assert.NoError(t, err, "Should handle HTTP to HTTPS redirect")
	
	// Verify file was created and has content
	assert.FileExists(t, tmpFile, "File should exist after redirect")
	
	content, err := os.ReadFile(tmpFile)
	assert.NoError(t, err, "Should be able to read file after redirect")
	assert.NotEmpty(t, content, "File should have content after redirect")
}