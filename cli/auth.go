package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

const keyringService = "mobilecli"
const keyringUser = "mobilenexthq.com"

const deviceFlowClientID = "ed38b523-56e8-4719-837b-7074fac152b5"
const deviceCodeURL = "https://app.mobilenexthq.com/login/device/code"
const deviceTokenURL = "https://app.mobilenexthq.com/login/device/token"
const deviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

const authHTTPTimeout = 30 * time.Second

var authHTTPClient = &http.Client{Timeout: authHTTPTimeout}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Error           string `json:"error,omitempty"`
}

type deviceTokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Error       string `json:"error,omitempty"`
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
	Long:  `Commands for managing authentication including login, logout, and token management.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to your account",
	Long:  `Opens the login page in your default browser to authenticate using a device code.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuthLogin()
	},
}

func requestDeviceCode() (*deviceCodeResponse, error) {
	body := strings.NewReader(fmt.Sprintf(`{"client_id":"%s"}`, deviceFlowClientID))
	resp, err := authHTTPClient.Post(deviceCodeURL, "application/json", body)
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result deviceCodeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse device code response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("device code error: %s", result.Error)
	}

	return &result, nil
}

func pollForToken(deviceCode string, interval, expiresIn int) (string, error) {
	pollInterval := time.Duration(interval) * time.Second
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)

		body := strings.NewReader(fmt.Sprintf(
			`{"client_id":"%s","device_code":"%s","grant_type":"%s"}`,
			deviceFlowClientID, deviceCode, deviceGrantType,
		))
		resp, err := authHTTPClient.Post(deviceTokenURL, "application/json", body)
		if err != nil {
			return "", fmt.Errorf("failed to poll for token: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("failed to read response: %w", err)
		}

		var tokenResp deviceTokenResponse
		if err := json.Unmarshal(respBody, &tokenResp); err != nil {
			return "", fmt.Errorf("failed to parse token response: %w", err)
		}

		switch tokenResp.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			pollInterval += time.Second
			continue
		case "expired_token":
			return "", fmt.Errorf("device code expired, please try again")
		case "":
			if tokenResp.AccessToken != "" {
				return tokenResp.AccessToken, nil
			}
			return "", fmt.Errorf("unexpected empty token response")
		default:
			return "", fmt.Errorf("token error: %s", tokenResp.Error)
		}
	}

	return "", fmt.Errorf("device code expired, please try again")
}

func runAuthLogin() error {
	codeResp, err := requestDeviceCode()
	if err != nil {
		return err
	}

	fmt.Printf("To log in, open this URL in your browser:\n\n\t%s\n\n", codeResp.VerificationURI)
	fmt.Printf("And enter the code: %s\n\n", codeResp.UserCode)
	fmt.Println("Waiting for authorization...")

	token, err := pollForToken(codeResp.DeviceCode, codeResp.Interval, codeResp.ExpiresIn)
	if err != nil {
		return err
	}

	if err := keyring.Set(keyringService, keyringUser, token); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	fmt.Println("Successfully logged in")
	return nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}
	return nil
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of your account",
	Long:  `Logs out of your current session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := keyring.Delete(keyringService, keyringUser); err != nil {
			if errors.Is(err, keyring.ErrNotFound) {
				fmt.Println("mobilecli is not logged in")
				return nil
			}
			return fmt.Errorf("failed to delete credentials: %w", err)
		}

		fmt.Println("Logged out successfully.")
		return nil
	},
}

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Display the current auth token",
	Long:  `Displays the authentication token for the current session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := keyring.Get(keyringService, keyringUser)
		if err != nil {
			return fmt.Errorf("no auth token found for mobilecli")
		}

		fmt.Println(token)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authTokenCmd)
}
