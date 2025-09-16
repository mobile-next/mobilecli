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
		absDestDir, err := filepath.Abs(destDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute dest dir: %w", err)
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		if !strings.HasPrefix(absPath, absDestDir+string(os.PathSeparator)) && absPath != absDestDir {
			return fmt.Errorf("path traversal attempt: %s resolves to %s", file.Name, absPath)
		}

		// Create directory tree
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		// Create directory for file if not exists
		if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
			return err
		}

		// Open source (zip) and destination files and copy, closing immediately
		rc, err := file.Open()
		if err != nil {
			return err
		}

		outFile, err := os.Create(path)
		if err != nil {
			rc.Close()
			return err
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			// Best-effort close before returning error
			rc.Close()
			outFile.Close()
			return err
		}

		// Close readers/writers promptly to avoid file descriptor buildup
		if err := rc.Close(); err != nil {
			outFile.Close()
			return err
		}

		if err := outFile.Close(); err != nil {
			return err
		}
	}

	return nil
}
