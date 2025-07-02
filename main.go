package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/server"
	"github.com/mobile-next/mobilecli/utils"
	"github.com/spf13/cobra"
)

const version = "dev"

var AppCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage applications on devices",
	Long:  `Install, uninstall, and manage applications on iOS and Android devices.`,
}

var (
	verbose bool

	// all commands
	deviceId string

	// for screenshot command
	screenshotOutputPath  string
	screenshotFormat      string
	screenshotJpegQuality int

	// for screencapture command
	screencaptureFormat string

	// for devices command
	platform   string
	deviceType string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "mobilecli",
	Short: "A cross-platform iOS/Android device automation tool",
	Long:  `A universal tool for managing iOS and Android devices`,
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List connected devices",
	Long:  `List all connected iOS and Android devices, both real devices and simulators/emulators.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.DevicesCommand()
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var screenshotCmd = &cobra.Command{
	Use:   "screenshot",
	Short: "Take a screenshot of a connected device",
	Long:  `Takes a screenshot of a specified device (using its ID) and saves it locally as a PNG file. Supports iOS (real/simulator) and Android (real/emulator).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.ScreenshotRequest{
			DeviceID:   deviceId,
			Format:     screenshotFormat,
			Quality:    screenshotJpegQuality,
			OutputPath: screenshotOutputPath,
		}

		response := commands.ScreenshotCommand(req)

		// Handle stdout output for binary data
		if screenshotOutputPath == "-" && response.Status == "ok" {
			if screenshotResp, ok := response.Data.(commands.ScreenshotResponse); ok && screenshotResp.Data != "" {
				// Write binary data to stdout
				imageBytes, err := base64.StdEncoding.DecodeString(screenshotResp.Data)
				if err != nil {
					return fmt.Errorf("failed to decode image data: %v", err)
				}
				_, err = os.Stdout.Write(imageBytes)
				if err != nil {
					return fmt.Errorf("failed to write to stdout: %v", err)
				}
				return nil
			}
		}

		// Print JSON response
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var screencaptureCmd = &cobra.Command{
	Use:   "screencapture",
	Short: "Stream screen capture from a connected device",
	Long:  `Streams MJPEG screen capture from a specified device to stdout. Only supports MJPEG format.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate format
		if screencaptureFormat != "mjpeg" {
			response := commands.NewErrorResponse(fmt.Errorf("format must be 'mjpeg' for screen capture"))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		// Find the target device
		targetDevice, err := commands.FindDeviceOrAutoSelect(deviceId)
		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error finding device: %v", err))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		// Start agent
		err = targetDevice.StartAgent()
		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error starting agent: %v", err))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		// Start screen capture and stream to stdout
		err = targetDevice.StartScreenCapture(screencaptureFormat, func(data []byte) bool {
			_, writeErr := os.Stdout.Write(data)
			if writeErr != nil {
				fmt.Fprintf(os.Stderr, "Error writing data: %v\n", writeErr)
				return false
			}
			return true
		})

		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error starting screen capture: %v", err))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		return nil
	},
}

var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot a connected device or simulator",
	Long:  `Reboots a specified device (using its ID). Supports iOS (real/simulator) and Android (real/emulator).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.RebootRequest{
			DeviceID: deviceId,
		}

		response := commands.RebootCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}

		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get device info",
	Long:  `Get detailed information about a connected device, such as OS, version, and screen size.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.InfoCommand(deviceId)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var ioCmd = &cobra.Command{
	Use:   "io",
	Short: "Input/output operations with devices",
	Long:  `Perform input/output operations like tapping, pressing buttons, and sending text to devices.`,
}

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "Manage applications on devices",
	Long:  `Launch, terminate, and manage applications on connected devices.`,
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server management commands",
	Long:  `Commands for managing the mobilecli server.`,
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the mobilecli server",
	Long:  `Starts the mobilecli server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		listenAddr := cmd.Flag("listen").Value.String()
		if listenAddr == "" {
			listenAddr = "localhost:12000"
		}

		enableCORS, _ := cmd.Flags().GetBool("cors")
		return server.StartServer(listenAddr, enableCORS)
	},
}

// io subcommands
var ioTapCmd = &cobra.Command{
	Use:   "tap [x,y]",
	Short: "Tap on a device screen at the given coordinates",
	Long:  `Sends a tap event to the specified device at the given x,y coordinates. Coordinates should be provided as a single string "x,y".`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		coordsStr := args[0]
		parts := strings.Split(coordsStr, ",")
		if len(parts) != 2 {
			response := commands.NewErrorResponse(fmt.Errorf("invalid coordinate format. Expected 'x,y', got '%s'", coordsStr))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		x, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
		y, errY := strconv.Atoi(strings.TrimSpace(parts[1]))

		if errX != nil || errY != nil {
			response := commands.NewErrorResponse(fmt.Errorf("invalid coordinate values. x and y must be integers. Got x='%s', y='%s'", parts[0], parts[1]))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		req := commands.TapRequest{
			DeviceID: deviceId,
			X:        x,
			Y:        y,
		}

		response := commands.TapCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var ioButtonCmd = &cobra.Command{
	Use:   "button [button_name]",
	Short: "Press a hardware button on a device",
	Long:  `Sends a hardware button press event to the specified device (e.g., "HOME", "VOLUME_UP", "VOLUME_DOWN", "POWER"). Button names are case-insensitive.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.ButtonRequest{
			DeviceID: deviceId,
			Button:   args[0],
		}

		response := commands.ButtonCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var ioTextCmd = &cobra.Command{
	Use:   "text [text]",
	Short: "Send text input to a device",
	Long:  `Sends text input to the currently focused element on the specified device.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.TextRequest{
			DeviceID: deviceId,
			Text:     args[0],
		}

		response := commands.TextCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var appsLaunchCmd = &cobra.Command{
	Use:   "launch [bundle_id]",
	Short: "Launch an app on a device",
	Long:  `Launches an app on the specified device using its bundle ID (e.g., "com.example.app").`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.AppRequest{
			DeviceID: deviceId,
			BundleID: args[0],
		}

		response := commands.LaunchAppCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var appsTerminateCmd = &cobra.Command{
	Use:   "terminate [bundle_id]",
	Short: "Terminate an app on a device",
	Long:  `Terminates an app on the specified device using its bundle ID (e.g., "com.example.app").`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.AppRequest{
			DeviceID: deviceId,
			BundleID: args[0],
		}

		response := commands.TerminateAppCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var appsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed apps on a device",
	Long:  `Lists all applications installed on the specified device.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.ListAppsRequest{
			DeviceID: deviceId,
		}

		response := commands.ListAppsCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var urlCmd = &cobra.Command{
	Use:   "url [url]",
	Short: "Open a URL on a device",
	Long:  `Opens a URL in the default browser on the specified device`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.URLRequest{
			DeviceID: deviceId,
			URL:      args[0],
		}

		response := commands.URLCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

func printJson(data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(jsonData))
}

func initConfig() {
	utils.SetVerbose(verbose)
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// add main commands
	rootCmd.AddCommand(devicesCmd)
	rootCmd.AddCommand(screenshotCmd)
	rootCmd.AddCommand(screencaptureCmd)
	rootCmd.AddCommand(rebootCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(ioCmd)
	rootCmd.AddCommand(appsCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(urlCmd)

	// add io subcommands
	ioCmd.AddCommand(ioTapCmd)
	ioCmd.AddCommand(ioButtonCmd)
	ioCmd.AddCommand(ioTextCmd)

	// add apps subcommands
	appsCmd.AddCommand(appsLaunchCmd)
	appsCmd.AddCommand(appsTerminateCmd)
	appsCmd.AddCommand(appsListCmd)

	// add server subcommands
	serverCmd.AddCommand(serverStartCmd)
	serverStartCmd.Flags().String("listen", "", "Address to listen on (e.g., 'localhost:12000' or '0.0.0.0:13000')")
	serverStartCmd.Flags().Bool("cors", false, "Enable CORS support")

	devicesCmd.Flags().StringVar(&platform, "platform", "", "target platform (ios or android)")
	devicesCmd.Flags().StringVar(&deviceType, "type", "", "filter by device type (real or simulator/emulator)")

	screenshotCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to take screenshot from")
	screenshotCmd.Flags().StringVarP(&screenshotOutputPath, "output", "o", "", "Output file path for screenshot (e.g., screen.png, or '-' for stdout)")
	screenshotCmd.Flags().StringVarP(&screenshotFormat, "format", "f", "png", "Output format for screenshot (png or jpeg)")
	screenshotCmd.Flags().IntVarP(&screenshotJpegQuality, "quality", "q", 90, "JPEG quality (1-100, only applies if format is jpeg)")

	screencaptureCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to capture from")
	screencaptureCmd.Flags().StringVarP(&screencaptureFormat, "format", "f", "mjpeg", "Output format for screen capture")

	rebootCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to reboot")

	infoCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to get info from")

	// io command flags
	ioTapCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to tap on")

	ioButtonCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to press button on")

	ioTextCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to send keys to")

	// apps command flags
	appsLaunchCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to launch app on")

	appsTerminateCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to terminate app on")

	appsListCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to list apps from")

	// url command flags
	urlCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to open URL on")
}

func main() {
	// enable microseconds in logs
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func findTargetDevice(deviceID string) (devices.ControllableDevice, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("--device flag is required")
	}

	allDevices, err := devices.GetAllControllableDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	for _, d := range allDevices {
		if d.ID() == deviceID {
			return d, nil
		}
	}

	return nil, fmt.Errorf("device with ID '%s' not found", deviceID)
}
