package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Unzip(zipPath string) (string, error) {

	tempDir, err := os.MkdirTemp("", "zip_unzip_")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Unzip the zip file
	if err := unzipFile(zipPath, tempDir); err != nil {
		return "", fmt.Errorf("failed to unzip zip file: %w", err)
	}

	return tempDir, nil
}

// unzipFile extracts a zip file to the specified destination
func unzipFile(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(destDir, file.Name)

		// Validate path to prevent zip slip attacks
		cleanPath := filepath.Clean(path)
		cleanDestDir := filepath.Clean(destDir)
		if !strings.HasPrefix(cleanPath, cleanDestDir+string(os.PathSeparator)) && cleanPath != cleanDestDir {
			return fmt.Errorf("path traversal attempt: %s resolves to %s", file.Name, cleanPath)
		}

		// Create directory tree
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, 0750)
			continue
		}

		// Create directory for file if not exists
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return err
		}

		// Create file
		outFile, err := os.Create(path)
		if err != nil {
			return err
		}

		// Open file in zip
		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		defer outFile.Close()
		defer rc.Close()

		_, err = io.Copy(outFile, rc)
		if err != nil {
			return err
		}

	}
	return nil
}
