package cli

import (
	"strings"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var fsCmd = &cobra.Command{
	Use:   "fs",
	Short: "Access device filesystem",
	Long:  `Push, pull, list, and manage files on a device or in an app's container.`,
}

var fsPushCmd = &cobra.Command{
	Use:   "push <local-path> <remote-path>",
	Short: "Push a file to the device or into an app's container",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.FsPushRequest{
			DeviceID:   deviceId,
			LocalPath:  absLocalPath(args[0]),
			RemotePath: args[1],
		}
		return runViaDaemon("cli.fs.push", req)
	},
}

var fsPullCmd = &cobra.Command{
	Use:   "pull <remote-path> <local-path>",
	Short: "Pull a file from the device or from an app's container",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.FsPullRequest{
			DeviceID:   deviceId,
			RemotePath: args[0],
			LocalPath:  absLocalPath(args[1]),
		}
		return runViaDaemon("cli.fs.pull", req)
	},
}

var fsLsCmd = &cobra.Command{
	Use:   "ls [bundle-id] [remote-path]",
	Short: "List files on the device or in an app's container",
	Args:  cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var bundleID, remotePath string
		switch len(args) {
		case 1:
			if strings.HasPrefix(args[0], "/") {
				remotePath = args[0]
			} else {
				bundleID = args[0]
			}
		case 2:
			bundleID = args[0]
			remotePath = args[1]
		}
		req := commands.FsListRequest{
			DeviceID:   deviceId,
			BundleID:   bundleID,
			RemotePath: remotePath,
		}
		return runViaDaemon("cli.fs.ls", req)
	},
}

var (
	fsMkdirParents bool
	fsRmRecursive  bool
)

var fsMkdirCmd = &cobra.Command{
	Use:   "mkdir [bundle-id] <remote-path>",
	Short: "Create a directory on the device or in an app's container",
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
		return runViaDaemon("cli.fs.mkdir", req)
	},
}

var fsRmCmd = &cobra.Command{
	Use:   "rm [bundle-id] <remote-path>",
	Short: "Remove a file or directory on the device or in an app's container",
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
		return runViaDaemon("cli.fs.rm", req)
	},
}

func init() {
	rootCmd.AddCommand(fsCmd)

	fsCmd.AddCommand(fsPushCmd)
	fsCmd.AddCommand(fsPullCmd)
	fsCmd.AddCommand(fsLsCmd)
	fsCmd.AddCommand(fsMkdirCmd)
	fsCmd.AddCommand(fsRmCmd)

	fsMkdirCmd.Flags().BoolVarP(&fsMkdirParents, "parents", "p", false, "Create parent directories as needed")
	fsRmCmd.Flags().BoolVarP(&fsRmRecursive, "recursive", "r", false, "Remove directories and their contents recursively")
}
