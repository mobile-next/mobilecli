package cli

import (
	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent management commands",
	Long:  `Commands for managing the on-device agent.`,
}

var agentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check agent installation status on a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.agent.status", commands.DeviceIDRequest{DeviceID: deviceId})
	},
}

var agentInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the agent on a device",
	Long:  `Installs the on-device agent on the specified device.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.agent.install", commands.AgentInstallRequest{
			DeviceID:            deviceId,
			Force:               agentForce,
			ProvisioningProfile: absLocalPath(agentProvisioningProfile),
		})
	},
}

var agentUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the agent from a device",
	Long:  `Removes the on-device agent from the specified device.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.agent.uninstall", commands.DeviceIDRequest{DeviceID: deviceId})
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.AddCommand(agentInstallCmd)
	agentCmd.AddCommand(agentStatusCmd)
	agentCmd.AddCommand(agentUninstallCmd)

	agentInstallCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to install the agent on")
	agentStatusCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to check")
	agentUninstallCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to uninstall the agent from")
	agentInstallCmd.Flags().BoolVar(&agentForce, "force", false, "force install even if agent is already installed")
	agentInstallCmd.Flags().StringVar(&agentProvisioningProfile, "provisioning-profile", "", "path to a .mobileprovision file to use for re-signing (required for real iOS devices)")
}
