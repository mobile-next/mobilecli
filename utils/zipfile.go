package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mholt/archiver"
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

// UnzipRepackWithProvision takes an IPA file path, unzips it to a temporary directory,
// adds or replaces the embedded.mobileprovision file, and then repackages it.
// It returns the path to the new IPA file or an error if the process fails.
func UnzipRepackWithProvision(ipaPath, provisionPath string) (string, error) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "ipa_repack_")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp dir when done

	// Unzip the IPA file
	if err := unzipFile(ipaPath, tempDir); err != nil {
		return "", fmt.Errorf("failed to unzip IPA: %w", err)
	}

	// Find the Payload directory and app directory
	payloadDir := filepath.Join(tempDir, "Payload")
	appDirs, err := filepath.Glob(filepath.Join(payloadDir, "*.app"))
	if err != nil || len(appDirs) == 0 {
		return "", fmt.Errorf("failed to find .app directory: %w", err)
	}
	appDir := appDirs[0]

	// Copy the provision file to the app directory
	destProvisionPath := filepath.Join(appDir, "embedded.mobileprovision")
	if err := CopyFile(provisionPath, destProvisionPath); err != nil {
		return "", fmt.Errorf("failed to copy provision file: %w", err)
	}

	// Create a new IPA file
	outputPath := ipaPath[:len(ipaPath)-4] + "_repacked.ipa"
	sourceToZip := filepath.Join(tempDir, "Payload")
	if err := archiver.Archive([]string{sourceToZip}, outputPath); err != nil {
		return "", fmt.Errorf("failed to create new IPA: %w", err)
	}

	return outputPath, nil
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

		// Create directory tree
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, os.ModePerm)
			continue
		}

		// Create directory for file if not exists
		if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
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
