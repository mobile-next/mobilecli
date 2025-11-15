package cli

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/mobile-next/mobilecli/utils"
	"github.com/spf13/cobra"
)

const version = "dev"

const logo = "\x1b[92m ███╗   ███╗  ██████╗  ██████╗  ██╗ ██╗      ███████╗  ██████╗ ██╗      ██╗\x1b[39m\n" +
	"\x1b[92m ████╗ ████║ ██╔═══██╗ ██╔══██╗ ██║ ██║      ██╔════╝ ██╔════╝ ██║      ██║\x1b[39m\n" +
	"\x1b[92m ██╔████╔██║ ██║   ██║ ██████╔╝ ██║ ██║      █████╗   ██║      ██║      ██║\x1b[39m\n" +
	"\x1b[92m ██║╚██╔╝██║ ██║   ██║ ██╔══██╗ ██║ ██║      ██╔══╝   ██║      ██║      ██║\x1b[39m\n" +
	"\x1b[92m ██║ ╚═╝ ██║ ╚██████╔╝ ██████╔╝ ██║ ███████╗ ███████  ╚██████╗ ███████╗ ██║\x1b[39m\n" +
	"\x1b[92m ╚═╝     ╚═╝  ╚═════╝  ╚═════╝  ╚═╝ ╚══════╝ ╚══════╝  ╚═════╝ ╚══════╝ ╚═╝\x1b[39m\n"

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "mobilecli",
	Short: "A cross-platform iOS/Android device automation tool",
	Long:  logo + "\n\nA universal tool for managing iOS and Android devices",
	Example: `DEVICE MANAGEMENT:
  # List all booted devices
  mobilecli devices

  # List all devices including offline ones
  mobilecli devices --include-offline --platform ios --type simulator

  # Boot an offline emulator/simulator device
  mobilecli device boot --device <device-id>

  # Shutdown a running emulator/simulator device
  mobilecli device shutdown --device <device-id>

  # Reboot a device
  mobilecli device reboot --device <device-id>

  # Get device info (OS, version, screen size)
  mobilecli device info --device <device-id>

  # Get/set device orientation
  mobilecli device orientation get --device <device-id>
  mobilecli device orientation set --device <device-id> landscape

APP MANAGEMENT:
  # Launch an app
  mobilecli apps launch --device <device-id> com.example.app

  # Terminate an app
  mobilecli apps terminate --device <device-id> com.example.app

  # List installed apps
  mobilecli apps list --device <device-id>

  # Install an app (.apk for Android, .ipa/.zip for iOS)
  mobilecli apps install --device <device-id> /path/to/app.apk

  # Uninstall an app
  mobilecli apps uninstall --device <device-id> com.example.app

SCREEN & MEDIA:
  # Take a screenshot
  mobilecli screenshot --device <device-id> -o screen.png

  # Take a JPEG screenshot with quality
  mobilecli screenshot --device <device-id> -o screen.jpg -f jpeg -q 85

  # Stream screen capture (MJPEG)
  mobilecli screencapture --device <device-id> -f mjpeg | ffplay -

INPUT/OUTPUT:
  # Tap at coordinates
  mobilecli io tap --device <device-id> 100,200

  # Long press at coordinates
  mobilecli io longpress --device <device-id> 100,200

  # Swipe from one point to another
  mobilecli io swipe --device <device-id> 100,200,300,400

  # Press hardware button (HOME, VOLUME_UP, VOLUME_DOWN, POWER)
  mobilecli io button --device <device-id> HOME

  # Send text input
  mobilecli io text --device <device-id> "Hello World"

UTILITIES:
  # Open a URL or deep link
  mobilecli url --device <device-id> https://example.com

  # Dump UI tree
  mobilecli dump ui --device <device-id>

  # Start HTTP server
  mobilecli server start --listen localhost:12000 --cors

COMMON FLAGS:
  --device <id>        Device ID (from 'mobilecli devices' command)
  --platform <name>    Filter by platform: ios or android
  --type <name>        Filter by type: real, simulator, or emulator
  --include-offline    Include offline devices in listing
  -o, --output <path>  Output file path (use '-' for stdout)
  -f, --format <fmt>   Format: png, jpeg, mjpeg
  -q, --quality <num>  JPEG quality (1-100)
  -v, --verbose        Enable verbose output
  --help               Show help for any command`,
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func initConfig() {
	utils.SetVerbose(verbose)
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVar(&deviceId, "device", "", "Device ID (get from 'mobilecli devices' command)")
}

// Execute runs the root command
func Execute() error {
	// enable microseconds in logs
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	return rootCmd.Execute()
}

// printJson is a helper function to print JSON responses
func printJson(data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(jsonData))
}
