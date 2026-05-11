package cli

import (
	"fmt"
	"strings"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var appsFsCmd = &cobra.Command{
	Use:   "fs",
	Short: "Access app container filesystem",
	Long:  `Push, pull, list, and manage files within an app's container filesystem.`,
}

var appsFsPushCmd = &cobra.Command{
	Use:   "push [bundle-id] <local-path> <remote-path>",
	Short: "Push a file into an app's container or to an absolute path",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		var bundleID, localPath, remotePath string
		if len(args) == 2 {
			localPath = args[0]
			remotePath = args[1]
		} else {
			bundleID = args[0]
			localPath = args[1]
			remotePath = args[2]
		}
		req := commands.FsPushRequest{
			DeviceID:   deviceId,
			BundleID:   bundleID,
			LocalPath:  localPath,
			RemotePath: remotePath,
		}
		response := commands.FsPushCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var appsFsPullCmd = &cobra.Command{
	Use:   "pull [bundle-id] <remote-path> <local-path>",
	Short: "Pull a file from an app's container or an absolute path",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		var bundleID, remotePath, localPath string
		if len(args) == 2 {
			remotePath = args[0]
			localPath = args[1]
		} else {
			bundleID = args[0]
			remotePath = args[1]
			localPath = args[2]
		}
		req := commands.FsPullRequest{
			DeviceID:   deviceId,
			BundleID:   bundleID,
			RemotePath: remotePath,
			LocalPath:  localPath,
		}
		response := commands.FsPullCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var appsFsLsCmd = &cobra.Command{
	Use:   "ls [bundle-id] [remote-path]",
	Short: "List files in an app's container or at an absolute path",
	Args:  cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var bundleID, remotePath string
		if len(args) == 0 {
			remotePath = "/"
		} else if len(args) == 1 && strings.HasPrefix(args[0], "/") {
			remotePath = args[0]
		} else {
			bundleID = args[0]
			if len(args) == 2 {
				remotePath = args[1]
			}
		}
		req := commands.FsListRequest{
			DeviceID:   deviceId,
			BundleID:   bundleID,
			RemotePath: remotePath,
		}
		response := commands.FsListCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var (
	fsMkdirParents bool
	fsRmRecursive  bool
)

var appsFsMkdirCmd = &cobra.Command{
	Use:   "mkdir [bundle-id] <remote-path>",
	Short: "Create a directory at an absolute path or in an app's container",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var bundleID, remotePath string
		if len(args) == 1 {
			remotePath = args[0]
		} else {
			bundleID = args[0]
			remotePath = args[1]
		}
		req := commands.FsMkdirRequest{
			DeviceID:   deviceId,
			BundleID:   bundleID,
			RemotePath: remotePath,
			Parents:    fsMkdirParents,
		}
		response := commands.FsMkdirCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var appsFsRmCmd = &cobra.Command{
	Use:   "rm [bundle-id] <remote-path>",
	Short: "Remove a file or directory from an absolute path or an app's container",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var bundleID, remotePath string
		if len(args) == 1 {
			remotePath = args[0]
		} else {
			bundleID = args[0]
			remotePath = args[1]
		}
		req := commands.FsRmRequest{
			DeviceID:   deviceId,
			BundleID:   bundleID,
			RemotePath: remotePath,
			Recursive:  fsRmRecursive,
		}
		response := commands.FsRmCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

func init() {
	appsCmd.AddCommand(appsFsCmd)

	appsFsCmd.AddCommand(appsFsPushCmd)
	appsFsCmd.AddCommand(appsFsPullCmd)
	appsFsCmd.AddCommand(appsFsLsCmd)
	appsFsCmd.AddCommand(appsFsMkdirCmd)
	appsFsCmd.AddCommand(appsFsRmCmd)

	appsFsPushCmd.Flags().StringVar(&deviceId, "device", "", "ID of the target device")
	appsFsPullCmd.Flags().StringVar(&deviceId, "device", "", "ID of the target device")
	appsFsLsCmd.Flags().StringVar(&deviceId, "device", "", "ID of the target device")
	appsFsMkdirCmd.Flags().StringVar(&deviceId, "device", "", "ID of the target device")
	appsFsMkdirCmd.Flags().BoolVarP(&fsMkdirParents, "parents", "p", false, "Create parent directories as needed")
	appsFsRmCmd.Flags().StringVar(&deviceId, "device", "", "ID of the target device")
	appsFsRmCmd.Flags().BoolVarP(&fsRmRecursive, "recursive", "r", false, "Remove directories and their contents recursively")
}
