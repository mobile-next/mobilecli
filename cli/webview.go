package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var webviewCmd = &cobra.Command{
	Use:   "webview",
	Short: "Inspect and interact with embedded webviews",
	Long:  `List, navigate, evaluate JavaScript, and inspect DOM content inside embedded webviews on a device.`,
}

var webviewListCmd = &cobra.Command{
	Use:   "list",
	Short: "List embedded webviews on a device",
	Long:  `Returns all embedded webviews currently visible in the foreground app. Browser apps (Safari, Chrome) are not included.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.WebViewListCommand(commands.WebViewListRequest{
			DeviceID: deviceId,
		})
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var webviewGotoCmd = &cobra.Command{
	Use:   "goto <id> <url>",
	Short: "Navigate a webview to a URL",
	Long:  `Navigates the specified webview to the given URL. The webview id comes from 'webview list'.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.WebViewGotoCommand(commands.WebViewGotoRequest{
			DeviceID:  deviceId,
			WebViewID: args[0],
			URL:       args[1],
		})
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var webviewReloadCmd = &cobra.Command{
	Use:   "reload <id>",
	Short: "Reload a webview",
	Long:  `Reloads the page currently loaded in the specified webview.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.WebViewReloadCommand(commands.WebViewReloadRequest{
			DeviceID:  deviceId,
			WebViewID: args[0],
		})
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var webviewBackCmd = &cobra.Command{
	Use:   "back <id>",
	Short: "Navigate a webview back",
	Long:  `Navigates the webview back in its history, equivalent to pressing the browser back button.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.WebViewGoBackCommand(commands.WebViewRequest{
			DeviceID:  deviceId,
			WebViewID: args[0],
		})
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var webviewForwardCmd = &cobra.Command{
	Use:   "forward <id>",
	Short: "Navigate a webview forward",
	Long:  `Navigates the webview forward in its history, equivalent to pressing the browser forward button.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.WebViewGoForwardCommand(commands.WebViewRequest{
			DeviceID:  deviceId,
			WebViewID: args[0],
		})
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var webviewEvalCmd = &cobra.Command{
	Use:   "eval <id> <expression>",
	Short: "Evaluate JavaScript in a webview",
	Long:  `Evaluates a JavaScript expression in the context of the specified webview and returns the result.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.WebViewEvaluateCommand(commands.WebViewEvaluateRequest{
			DeviceID:   deviceId,
			WebViewID:  args[0],
			Expression: args[1],
		})
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var webviewWaitCmd = &cobra.Command{
	Use:   "wait <id>",
	Short: "Wait for a webview to finish loading",
	Long:  `Waits for the webview to reach the specified load state before returning.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.WebViewWaitForLoadStateCommand(commands.WebViewWaitForLoadStateRequest{
			DeviceID:  deviceId,
			WebViewID: args[0],
			State:     webviewWaitState,
			Timeout:   webviewWaitTimeout,
		})
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

// ─── Convenience commands built on evaluate ───────────────────

var webviewURLCmd = &cobra.Command{
	Use:   "url <id>",
	Short: "Print the current URL of a webview",
	Long:  `Prints the current URL loaded in the specified webview.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.WebViewEvaluateCommand(commands.WebViewEvaluateRequest{
			DeviceID:   deviceId,
			WebViewID:  args[0],
			Expression: "return location.href",
		})
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var webviewTitleCmd = &cobra.Command{
	Use:   "title <id>",
	Short: "Print the title of a webview",
	Long:  `Prints the document title of the page currently loaded in the specified webview.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.WebViewEvaluateCommand(commands.WebViewEvaluateRequest{
			DeviceID:   deviceId,
			WebViewID:  args[0],
			Expression: "return document.title",
		})
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var webviewContentCmd = &cobra.Command{
	Use:   "content <id>",
	Short: "Dump the HTML content of a webview",
	Long:  `Returns the full outer HTML of the page currently loaded in the specified webview.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.WebViewContentCommand(commands.WebViewRequest{
			DeviceID:  deviceId,
			WebViewID: args[0],
		})
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var webviewQueryCmd = &cobra.Command{
	Use:   "query <id> <selector>",
	Short: "Query DOM elements in a webview",
	Long:  `Finds elements matching a CSS selector and returns their tag, text, id, and value. Useful for inspecting webview content.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		selector := args[1]
		expression := fmt.Sprintf(
			`Array.from(document.querySelectorAll(%q)).map(el => ({`+
				`tag: el.tagName.toLowerCase(),`+
				`text: (el.textContent || "").trim().slice(0, 200),`+
				`id: el.id || null,`+
				`class: el.className || null,`+
				`value: el.value || null,`+
				`href: el.href || null`+
				`}))`,
			selector,
		)
		response := commands.WebViewEvaluateCommand(commands.WebViewEvaluateRequest{
			DeviceID:   deviceId,
			WebViewID:  args[0],
			Expression: expression,
		})
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(webviewCmd)

	webviewCmd.AddCommand(webviewListCmd)
	webviewCmd.AddCommand(webviewGotoCmd)
	webviewCmd.AddCommand(webviewReloadCmd)
	webviewCmd.AddCommand(webviewBackCmd)
	webviewCmd.AddCommand(webviewForwardCmd)
	webviewCmd.AddCommand(webviewEvalCmd)
	webviewCmd.AddCommand(webviewWaitCmd)
	webviewCmd.AddCommand(webviewURLCmd)
	webviewCmd.AddCommand(webviewTitleCmd)
	webviewCmd.AddCommand(webviewContentCmd)
	webviewCmd.AddCommand(webviewQueryCmd)

	webviewListCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device")
	webviewGotoCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device")
	webviewReloadCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device")
	webviewBackCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device")
	webviewForwardCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device")
	webviewEvalCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device")
	webviewWaitCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device")
	webviewWaitCmd.Flags().StringVar(&webviewWaitState, "state", "load", `load state to wait for: "load" or "domcontentloaded"`)
	webviewWaitCmd.Flags().IntVar(&webviewWaitTimeout, "timeout", 0, "maximum time to wait in milliseconds (0 = default)")
	webviewURLCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device")
	webviewTitleCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device")
	webviewContentCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device")
	webviewQueryCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device")
}
