package cli

import (
	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Device management commands",
	Long:  `Commands for managing individual devices including rebooting and getting device information.`,
}

var deviceRebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot a connected device or simulator",
	Long:  `Reboots a specified device (using its ID). Supports iOS (real/simulator) and Android (real/emulator).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.RebootRequest{
			DeviceID: deviceId,
		}

		return runViaDaemon("cli.device.reboot", req)
	},
}

var deviceInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get device info",
	Long:  `Get detailed information about a connected device, such as OS, version, and screen size.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.device.info", commands.DeviceIDRequest{DeviceID: deviceId})
	},
}

var orientationCmd = &cobra.Command{
	Use:   "orientation",
	Short: "Device orientation commands",
	Long:  `Commands for getting and setting device orientation.`,
}

var orientationGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get current device orientation",
	Long:  `Get the current orientation of the device (portrait or landscape).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.OrientationGetRequest{
			DeviceID: deviceId,
		}

		return runViaDaemon("cli.device.orientation.get", req)
	},
}

var orientationSetCmd = &cobra.Command{
	Use:   "set [orientation]",
	Short: "Set device orientation",
	Long:  `Set the device orientation to portrait or landscape.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.OrientationSetRequest{
			DeviceID:    deviceId,
			Orientation: args[0],
		}

		return runViaDaemon("cli.device.orientation.set", req)
	},
}

var deviceFoldCmd = &cobra.Command{
	Use:   "fold [folded|half-open|open|degrees]",
	Short: "Fold or unfold a foldable device",
	Long:  `Sets the hinge of a foldable device to "folded" (0°), "half-open" (90°), "open" (180°), or an angle between 0 and 180. Supported on foldable iOS simulators and Android emulators.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.FoldRequest{
			DeviceID: deviceId,
			State:    args[0],
		}

		return runViaDaemon("cli.device.fold", req)
	},
}

var voiceOverCmd = &cobra.Command{
	Use:   "voiceover",
	Short: "VoiceOver commands",
	Long:  `Commands for turning VoiceOver on and off, reading what it speaks, and moving its cursor. Supported on iOS 27+ simulators and real devices. VoiceOver stays on after mobilecli exits, and while it is on a tap focuses an element instead of activating it.`,
}

var voiceOverGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get whether VoiceOver is on",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.device.voiceover.get", commands.VoiceOverGetRequest{DeviceID: deviceId})
	},
}

var voiceOverSetCmd = &cobra.Command{
	Use:   "set [on|off]",
	Short: "Turn VoiceOver on or off",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.device.voiceover.set", commands.VoiceOverSetRequest{DeviceID: deviceId, State: args[0]})
	},
}

var voiceOverSpeechCmd = &cobra.Command{
	Use:   "speech",
	Short: "Read what VoiceOver speaks for the focused element",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.device.voiceover.speech", commands.VoiceOverSpeechRequest{DeviceID: deviceId})
	},
}

var voiceOverMoveCmd = &cobra.Command{
	Use:   "move [next|previous|in|out]",
	Short: "Move the VoiceOver cursor and read what it speaks",
	Long:  `Moves the VoiceOver cursor to the next or previous element, or into or out of a container, and prints what VoiceOver speaks there. "in" and "out" need VoiceOver's Navigation Style set to Groups.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.device.voiceover.move", commands.VoiceOverMoveRequest{DeviceID: deviceId, Direction: args[0]})
	},
}

var deviceBootCmd = &cobra.Command{
	Use:   "boot",
	Short: "Boot a simulator or emulator",
	Long:  `Boots a specified offline simulator or emulator.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.BootRequest{
			DeviceID: deviceId,
		}

		return runViaDaemon("cli.device.boot", req)
	},
}

var deviceShutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Shutdown a simulator or emulator",
	Long:  `Shuts down a specified simulator or emulator.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.ShutdownRequest{
			DeviceID: deviceId,
		}

		return runViaDaemon("cli.device.shutdown", req)
	},
}

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Device settings commands",
	Long:  `Commands for applying device-level settings such as animations.`,
}

var settingsAnimations string
var settingsAppearance string

var settingsApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply device settings",
	Long:  `Apply device-level settings. Example: mobilecli device settings apply --animations=off --appearance=dark`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.ApplySettingsRequest{
			DeviceID: deviceId,
		}

		if cmd.Flags().Changed("animations") {
			req.Animations = &settingsAnimations
		}

		if cmd.Flags().Changed("appearance") {
			req.Appearance = &settingsAppearance
		}

		return runViaDaemon("cli.device.settings.apply", req)
	},
}

func init() {
	rootCmd.AddCommand(deviceCmd)

	// add device subcommands
	deviceCmd.AddCommand(deviceRebootCmd)
	deviceCmd.AddCommand(deviceInfoCmd)
	deviceCmd.AddCommand(deviceBootCmd)
	deviceCmd.AddCommand(deviceShutdownCmd)
	deviceCmd.AddCommand(orientationCmd)
	deviceCmd.AddCommand(deviceFoldCmd)
	deviceCmd.AddCommand(voiceOverCmd)
	deviceCmd.AddCommand(settingsCmd)

	// add orientation subcommands
	orientationCmd.AddCommand(orientationGetCmd)
	orientationCmd.AddCommand(orientationSetCmd)

	voiceOverCmd.AddCommand(voiceOverGetCmd)
	voiceOverCmd.AddCommand(voiceOverSetCmd)
	voiceOverCmd.AddCommand(voiceOverSpeechCmd)
	voiceOverCmd.AddCommand(voiceOverMoveCmd)

	// add settings subcommands
	settingsCmd.AddCommand(settingsApplyCmd)

	// device command flags
	deviceRebootCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to reboot")
	deviceInfoCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to get info from")
	deviceBootCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to boot")
	deviceShutdownCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to shutdown")
	orientationGetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to get orientation from")
	orientationSetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to set orientation on")
	deviceFoldCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to fold or unfold")
	voiceOverGetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to get the VoiceOver state from")
	voiceOverSetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to turn VoiceOver on or off")
	voiceOverSpeechCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to read VoiceOver speech from")
	voiceOverMoveCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to move the VoiceOver cursor on")
	settingsApplyCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to apply settings to")
	settingsApplyCmd.Flags().StringVar(&settingsAnimations, "animations", "", "Toggle system animations: 'on' or 'off'")
	settingsApplyCmd.Flags().StringVar(&settingsAppearance, "appearance", "", "System appearance: 'light' or 'dark'")
}
