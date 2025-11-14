package utils

import (
	"fmt"
	"os/exec"
)

// ModifyPlistInput contains parameters for modifying a plist file
type ModifyPlistInput struct {
	PlistPath string
	Key       string
	Value     string
}

// ModifyPlist modifies a plist file using plutil to add or replace a key-value pair
func ModifyPlist(input ModifyPlistInput) error {
	// try to replace first (if key exists)
	cmd := exec.Command("plutil", "-replace", input.Key, "-string", input.Value, input.PlistPath)
	_, err := cmd.CombinedOutput()
	if err != nil {
		// if replace failed, try to insert (key doesn't exist)
		cmd = exec.Command("plutil", "-insert", input.Key, "-string", input.Value, input.PlistPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to modify plist: %w\n%s", err, output)
		}
	}

	return nil
}

// AddBundleIconFilesToPlist adds the CFBundleIconFiles array to the plist
func AddBundleIconFilesToPlist(plistPath string) error {
	// insert CFBundleIconFiles as array
	cmd := exec.Command("plutil", "-insert", "CFBundleIconFiles", "-array", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to insert CFBundleIconFiles: %w\n%s", err, output)
	}

	// insert AppIcon.png as first element in the array
	cmd = exec.Command("plutil", "-insert", "CFBundleIconFiles.0", "-string", "AppIcon.png", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to insert AppIcon.png: %w\n%s", err, output)
	}

	return nil
}
