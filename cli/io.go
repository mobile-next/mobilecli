package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var ioCmd = &cobra.Command{
	Use:   "io",
	Short: "Input/output operations with devices",
	Long:  `Perform input/output operations like tapping, pressing buttons, sending text, and reading or writing the device clipboard.`,
}

// screenPoint is where a touch command should land: either explicit
// coordinates or an element ref ("@e5") to resolve against the current screen.
type screenPoint struct {
	X   int
	Y   int
	Ref string
}

// parseScreenPoint reads the single argument tap and longpress take: "x,y" or
// an element ref from the latest "dump ui".
func parseScreenPoint(arg string) (screenPoint, error) {
	if strings.HasPrefix(arg, "@") {
		return screenPoint{Ref: arg}, nil
	}

	parts := strings.Split(arg, ",")
	if len(parts) != 2 {
		return screenPoint{}, fmt.Errorf("invalid target format. Expected 'x,y' coordinates or an element ref like '@e15', got '%s'", arg)
	}

	x, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
	y, errY := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errX != nil || errY != nil {
		return screenPoint{}, fmt.Errorf("invalid coordinate values. x and y must be integers. Got x='%s', y='%s'", parts[0], parts[1])
	}

	return screenPoint{X: x, Y: y}, nil
}

var ioTapCmd = &cobra.Command{
	Use:   "tap [x,y | @ref]",
	Short: "Tap on a device screen at the given coordinates or element ref",
	Long:  `Sends a tap event to the specified device at the given x,y coordinates ("x,y"), or at the center of an element ref from the latest "dump ui" (e.g. "@e5").`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target, err := parseScreenPoint(args[0])
		if err != nil {
			response := commands.NewErrorResponse(err)
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		req := commands.TapRequest{
			DeviceID: deviceId,
			X:        target.X,
			Y:        target.Y,
			Ref:      target.Ref,
		}

		return runViaDaemon("cli.io.tap", req)
	},
}

var longPressDuration int
var swipeDuration int
var pinchDirection string
var pinchDistance int
var pinchDuration int

var ioLongPressCmd = &cobra.Command{
	Use:   "longpress [x,y | @ref]",
	Short: "Long press on a device screen at the given coordinates or element ref",
	Long:  `Sends a long press event to the specified device at the given x,y coordinates ("x,y"), or at the center of an element ref from the latest "dump ui" (e.g. "@e5").`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target, err := parseScreenPoint(args[0])
		if err != nil {
			response := commands.NewErrorResponse(err)
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		req := commands.LongPressRequest{
			DeviceID: deviceId,
			X:        target.X,
			Y:        target.Y,
			Duration: longPressDuration,
			Ref:      target.Ref,
		}

		return runViaDaemon("cli.io.longpress", req)
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

		return runViaDaemon("cli.io.button", req)
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

		return runViaDaemon("cli.io.text", req)
	},
}

var ioKeysCmd = &cobra.Command{
	Use:   "keys [key-combo...]",
	Short: "Press keyboard keys with optional modifiers on a device",
	Long: `Presses one or more key combinations on the specified device, in order. A combo is a key with optional modifiers, e.g. "cmd+a", "ctrl+shift+z", "backspace".

Keys name physical keys and are case-insensitive: "cmd+A" is the same as "cmd+a" (the A key, i.e. select-all), not Shift+A. Use "shift+a" to hold shift.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.KeysRequest{
			DeviceID: deviceId,
			Keys:     args,
		}

		return runViaDaemon("cli.io.keys", req)
	},
}

var ioSwipeCmd = &cobra.Command{
	Use:   "swipe [x1,y1,x2,y2]",
	Short: "Swipe on a device screen from one point to another",
	Long:  `Sends a swipe gesture to the specified device from coordinates x1,y1 to x2,y2. Coordinates should be provided as a single string "x1,y1,x2,y2".`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		coordsStr := args[0]
		parts := strings.Split(coordsStr, ",")
		if len(parts) != 4 {
			response := commands.NewErrorResponse(fmt.Errorf("invalid coordinate format. Expected 'x1,y1,x2,y2', got '%s'", coordsStr))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		x1, errX1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		y1, errY1 := strconv.Atoi(strings.TrimSpace(parts[1]))
		x2, errX2 := strconv.Atoi(strings.TrimSpace(parts[2]))
		y2, errY2 := strconv.Atoi(strings.TrimSpace(parts[3]))

		if errX1 != nil || errY1 != nil || errX2 != nil || errY2 != nil {
			response := commands.NewErrorResponse(fmt.Errorf("invalid coordinate values. x1, y1, x2, y2 must be integers. Got x1='%s', y1='%s', x2='%s', y2='%s'", parts[0], parts[1], parts[2], parts[3]))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		req := commands.SwipeRequest{
			DeviceID: deviceId,
			X1:       x1,
			Y1:       y1,
			X2:       x2,
			Y2:       y2,
			Duration: swipeDuration,
		}

		return runViaDaemon("cli.io.swipe", req)
	},
}

var ioPinchCmd = &cobra.Command{
	Use:   "pinch [x,y]",
	Short: "Pinch on a device screen with two fingers to zoom in or out",
	Long:  `Sends a two-finger pinch to the specified device, centered at the given x,y coordinates or at the center of the screen when omitted. "--direction out" spreads the fingers apart (zoom in); "--direction in" brings them together (zoom out). Coordinates should be provided as a single string "x,y".`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.PinchRequest{
			DeviceID:  deviceId,
			Direction: pinchDirection,
			Distance:  pinchDistance,
			Duration:  pinchDuration,
		}

		if len(args) == 1 {
			coordsStr := args[0]
			parts := strings.Split(coordsStr, ",")
			if len(parts) != 2 {
				response := commands.NewErrorResponse(fmt.Errorf("invalid coordinate format. Expected 'x,y', got '%s'", coordsStr))
				printJson(response)
				return fmt.Errorf("%s", response.Error)
			}

			x, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
			y, errY := strconv.Atoi(strings.TrimSpace(parts[1]))

			if errX != nil || errY != nil {
				response := commands.NewErrorResponse(fmt.Errorf("invalid coordinate values. x and y must be integers. Got x='%s', y='%s'", parts[0], parts[1]))
				printJson(response)
				return fmt.Errorf("%s", response.Error)
			}

			req.X = x
			req.Y = y
		}

		return runViaDaemon("cli.io.pinch", req)
	},
}

func init() {
	rootCmd.AddCommand(ioCmd)

	// add io subcommands
	ioCmd.AddCommand(ioTapCmd)
	ioCmd.AddCommand(ioLongPressCmd)
	ioCmd.AddCommand(ioButtonCmd)
	ioCmd.AddCommand(ioTextCmd)
	ioCmd.AddCommand(ioKeysCmd)
	ioCmd.AddCommand(ioSwipeCmd)
	ioCmd.AddCommand(ioPinchCmd)

	// io command flags
	ioTapCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to tap on")
	ioLongPressCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to long press on")
	ioLongPressCmd.Flags().IntVar(&longPressDuration, "duration", 500, "duration of the long press in milliseconds")
	ioSwipeCmd.Flags().IntVar(&swipeDuration, "duration", 0, "duration of the swipe in milliseconds (0 uses the platform default)")
	ioButtonCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to press button on")
	ioTextCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to send keys to")
	ioKeysCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to press keys on")
	ioSwipeCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to swipe on")
	ioPinchCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to pinch on")
	ioPinchCmd.Flags().StringVar(&pinchDirection, "direction", "", "\"in\" to zoom out or \"out\" to zoom in (required)")
	ioPinchCmd.Flags().IntVar(&pinchDistance, "distance", 200, "pixels each finger travels")
	ioPinchCmd.Flags().IntVar(&pinchDuration, "duration", 300, "duration of the finger movement in milliseconds")
	cobra.CheckErr(ioPinchCmd.MarkFlagRequired("direction"))
}
