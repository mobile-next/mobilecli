package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilectl/devices"
	"github.com/mobile-next/mobilectl/utils"
	"github.com/spf13/cobra"
)

const version = "dev"

var AppCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage applications on devices",
	Long:  `Install, uninstall, and manage applications on iOS and Android devices.`,
}

var (
	deviceType string
)

var (
	verbose               bool
	jsonOutput            bool
	deviceId              string
	platform              string
	screenshotOutputPath  string
	screenshotFormat      string
	screenshotJpegQuality int
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "mobilectl",
	Short: "A cross-platform iOS/Android device automation tool",
	Long:  `A universal tool for managing iOS and Android devices`,
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	Version: version,
}

// For JSON marshaling of ControllableDevice interface
type deviceJSON struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Type     string `json:"type"`
}

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List connected devices",
	Long:  `List all connected iOS and Android devices, both real devices and simulators/emulators.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		allDevices, err := devices.GetAllControllableDevices()
		if err != nil {
			log.Printf("Warning: Encountered errors while listing some devices: %v", err)
		}

		if err != nil {
			return err
		} else if len(allDevices) == 0 {
			fmt.Println("No devices found.")
			return nil
		}

		jsonDevices := make([]deviceJSON, len(allDevices))
		for i, d := range allDevices {
			jsonDevices[i] = deviceJSON{
				ID:       d.ID(),
				Name:     d.Name(),
				Platform: d.Platform(),
				Type:     d.DeviceType(),
			}
		}

		printJson(jsonDevices)
		return nil
	},
}

var screenshotCmd = &cobra.Command{
	Use:   "screenshot",
	Short: "Take a screenshot of a connected device",
	Long:  `Takes a screenshot of a specified device (using its ID) and saves it locally as a PNG file. Supports iOS (real/simulator) and Android (real/emulator).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDevice, err := findTargetDevice(deviceId)
		if err != nil {
			return err
		}

		// Validate format
		screenshotFormat = strings.ToLower(screenshotFormat)
		if screenshotFormat != "png" && screenshotFormat != "jpeg" {
			return fmt.Errorf("invalid format '%s'. Supported formats are 'png' and 'jpeg'", screenshotFormat)
		}

		// Validate JPEG quality if format is jpeg
		if screenshotFormat == "jpeg" {
			if screenshotJpegQuality < 1 || screenshotJpegQuality > 100 {
				return fmt.Errorf("invalid JPEG quality '%d'. Must be between 1 and 100", screenshotJpegQuality)
			}
		}

		err = targetDevice.StartAgent()
		if err != nil {
			return err
		}

		log.Printf("Attempting to take screenshot for %s device with ID: %s (Name: %s)", targetDevice.Platform(), targetDevice.ID(), targetDevice.Name())

		imageBytes, err := targetDevice.TakeScreenshot()
		if err != nil {
			return err
		}

		if screenshotFormat == "jpeg" {
			convertedBytes, err := utils.ConvertPngToJpeg(imageBytes, screenshotJpegQuality)
			if err != nil {
				return err
			}
			imageBytes = convertedBytes
			log.Printf("Converted screenshot to JPEG format with quality %d.", screenshotJpegQuality)
		}

		if screenshotOutputPath == "-" {
			_, err = os.Stdout.Write(imageBytes)
			if err != nil {
				return err
			}
			log.Printf("Screenshot for device %s written to stdout as %s.", targetDevice.ID(), screenshotFormat)
		} else {
			var finalPath string
			if screenshotOutputPath != "" {
				finalPath, err = filepath.Abs(screenshotOutputPath)
				if err != nil {
					return err
				}
			} else {
				// Default filename generation
				timestamp := time.Now().Format("20060102150405")
				safeDeviceID := strings.ReplaceAll(targetDevice.ID(), ":", "_")
				extension := "png"
				if screenshotFormat == "jpeg" {
					extension = "jpg"
				}
				fileName := fmt.Sprintf("screenshot-%s-%s.%s", safeDeviceID, timestamp, extension)
				finalPath, err = filepath.Abs("./" + fileName)
				if err != nil {
					return err
				}
			}

			err = os.WriteFile(finalPath, imageBytes, 0o644)
			if err != nil {
				return err
			}
			log.Printf("Screenshot for device %s saved to %s as %s.", targetDevice.ID(), finalPath, screenshotFormat)
		}

		return nil
	},
}

var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot a connected device or simulator",
	Long:  `Reboots a specified device (using its ID). Supports iOS (real/simulator) and Android (real/emulator).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDevice, err := findTargetDevice(deviceId)
		if err != nil {
			return err
		}

		log.Printf("Attempting to reboot %s device with ID: %s (Name: %s)", targetDevice.Platform(), targetDevice.ID(), targetDevice.Name())

		err = targetDevice.Reboot()
		if err != nil {
			return err
		}

		log.Printf("Reboot command processed for device %s. Check device for status.", targetDevice.ID())
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

// IO subcommands
var ioTapCmd = &cobra.Command{
	Use:   "tap [x,y]",
	Short: "Tap on a device screen at the given coordinates",
	Long:  `Sends a tap event to the specified device at the given x,y coordinates. Coordinates should be provided as a single string "x,y".`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDevice, err := findTargetDevice(deviceId)
		if err != nil {
			return err
		}

		coordsStr := args[0]
		parts := strings.Split(coordsStr, ",")
		if len(parts) != 2 {
			return fmt.Errorf("invalid coordinate format. Expected 'x,y', got '%s'", coordsStr)
		}

		x, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
		y, errY := strconv.Atoi(strings.TrimSpace(parts[1]))

		if errX != nil || errY != nil {
			return fmt.Errorf("invalid coordinate values. x and y must be integers. Got x='%s', y='%s'", parts[0], parts[1])
		}

		if x < 0 || y < 0 {
			return fmt.Errorf("x and y coordinates must be non-negative, got x=%d, y=%d", x, y)
		}

		err = targetDevice.StartAgent()
		if err != nil {
			return fmt.Errorf("failed to start agent on device %s: %w", targetDevice.ID(), err)
		}

		log.Printf("Attempting to tap on %s device '%s' at (%d,%d)", targetDevice.Platform(), targetDevice.ID(), x, y)

		err = targetDevice.Tap(x, y)
		if err != nil {
			return fmt.Errorf("failed to tap on device %s: %w", targetDevice.ID(), err)
		}

		log.Printf("Tap command processed for device %s at (%d,%d).", targetDevice.ID(), x, y)
		return nil
	},
}

var ioButtonCmd = &cobra.Command{
	Use:   "button [button_name]",
	Short: "Press a hardware button on a device",
	Long:  `Sends a hardware button press event to the specified device (e.g., "HOME", "VOLUME_UP", "VOLUME_DOWN", "POWER"). Button names are case-insensitive.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDevice, err := findTargetDevice(deviceId)
		if err != nil {
			return err
		}

		buttonName := args[0]

		err = targetDevice.StartAgent()
		if err != nil {
			return err
		}

		err = targetDevice.PressButton(buttonName)
		if err != nil {
			return err
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
		targetDevice, err := findTargetDevice(deviceId)
		if err != nil {
			return err
		}

		text := args[0]

		err = targetDevice.StartAgent()
		if err != nil {
			return err
		}

		err = targetDevice.SendKeys(text)
		if err != nil {
			return err
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
		targetDevice, err := findTargetDevice(deviceId)
		if err != nil {
			return err
		}

		bundleID := args[0]

		err = targetDevice.StartAgent()
		if err != nil {
			return err
		}

		err = targetDevice.LaunchApp(bundleID)
		if err != nil {
			return err
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
		targetDevice, err := findTargetDevice(deviceId)
		if err != nil {
			return err
		}

		bundleID := args[0]

		err = targetDevice.StartAgent()
		if err != nil {
			return err
		}

		err = targetDevice.TerminateApp(bundleID)
		if err != nil {
			return err
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
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")

	// add main commands
	rootCmd.AddCommand(devicesCmd)
	rootCmd.AddCommand(screenshotCmd)
	rootCmd.AddCommand(rebootCmd)
	rootCmd.AddCommand(ioCmd)
	rootCmd.AddCommand(appsCmd)

	// add io subcommands
	ioCmd.AddCommand(ioTapCmd)
	ioCmd.AddCommand(ioButtonCmd)
	ioCmd.AddCommand(ioTextCmd)

	// add apps subcommands
	appsCmd.AddCommand(appsLaunchCmd)
	appsCmd.AddCommand(appsTerminateCmd)

	devicesCmd.Flags().StringVar(&platform, "platform", "", "target platform (ios or android)")
	devicesCmd.Flags().StringVar(&deviceType, "type", "", "filter by device type (real or simulator/emulator)")

	screenshotCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to take screenshot from")
	screenshotCmd.Flags().StringVarP(&screenshotOutputPath, "output", "o", "", "Output file path for screenshot (e.g., screen.png, or '-' for stdout)")
	screenshotCmd.Flags().StringVarP(&screenshotFormat, "format", "f", "png", "Output format for screenshot (png or jpeg)")
	screenshotCmd.Flags().IntVarP(&screenshotJpegQuality, "quality", "q", 90, "JPEG quality (1-100, only applies if format is jpeg)")
	screenshotCmd.MarkFlagRequired("device")

	rebootCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to reboot")
	rebootCmd.MarkFlagRequired("device")

	// io command flags
	ioTapCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to tap on")
	ioTapCmd.MarkFlagRequired("device")

	ioButtonCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to press button on")
	ioButtonCmd.MarkFlagRequired("device")

	ioTextCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to send keys to")
	ioTextCmd.MarkFlagRequired("device")

	// apps command flags
	appsLaunchCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to launch app on")
	appsLaunchCmd.MarkFlagRequired("device")

	appsTerminateCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to terminate app on")
	appsTerminateCmd.MarkFlagRequired("device")
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
