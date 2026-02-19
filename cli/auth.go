package cli

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"

	"github.com/mobile-next/mobilecli/server"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

const keyringService = "mobilecli"
const keyringUser = "mobilenexthq.com"

const cognitoClientID = "26epocf8ss83d7uj8trmr6ktvn"
const cognitoTokenURL = "https://auth.mobilenexthq.com/oauth2/token"
const cognitoRedirectURI = "https://mobilenexthq.com/oauth/callback/"
const apiTokenURL = "https://api.mobilenexthq.com/auth/token"

type cognitoTokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type apiTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
	Long:  `Commands for managing authentication including login, logout, and token management.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to your account",
	Long:  `Opens the login page in your default browser to authenticate.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// generate csrf nonce
		nonceBytes := make([]byte, 16)
		if _, err := rand.Read(nonceBytes); err != nil {
			return fmt.Errorf("failed to generate csrf nonce: %w", err)
		}
		nonce := hex.EncodeToString(nonceBytes)

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("failed to start callback server: %w", err)
		}
		port := listener.Addr().(*net.TCPAddr).Port

		callbackErr := make(chan error, 1)
		mux := http.NewServeMux()
		srv := &http.Server{Handler: mux}

		mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
			err := handleOAuthCallback(r, nonce)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintln(w, err.Error())
			} else {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintln(w, "<html><body><h2>Login successful!</h2><p>You can close this window.</p></body></html>")
			}
			callbackErr <- err
			go srv.Shutdown(context.Background())
		})

		go func() {
			if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
				callbackErr <- fmt.Errorf("callback server error: %w", err)
			}
		}()

		loginURL := fmt.Sprintf(
			"https://mobilenexthq.com/oauth/login/?redirectUri=http://localhost:%d/oauth/callback&csrf=%s&agent=mobilecli&agentVersion=%s",
			port, nonce, server.Version,
		)

		fmt.Printf("Your browser has been opened to visit:\n\n\t%s\n\n", loginURL)

		var openCmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			openCmd = exec.Command("open", loginURL)
		case "linux":
			openCmd = exec.Command("xdg-open", loginURL)
		case "windows":
			openCmd = exec.Command("cmd", "/c", "start", loginURL)
		default:
			return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
		}

		if err := openCmd.Run(); err != nil {
			return fmt.Errorf("failed to open browser: %w", err)
		}

		if err := <-callbackErr; err != nil {
			return err
		}

		fmt.Println("✅ Successfully logged in")
		return nil
	},
}

func handleOAuthCallback(r *http.Request, nonce string) error {
	stateParam := r.URL.Query().Get("state")
	stateJSON, err := base64.StdEncoding.DecodeString(stateParam)
	if err != nil {
		stateJSON, err = base64.RawURLEncoding.DecodeString(stateParam)
	}
	if err != nil {
		return fmt.Errorf("invalid state parameter: %w", err)
	}

	var state struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return fmt.Errorf("invalid state parameter: %w", err)
	}

	if state.CSRF != nonce {
		return fmt.Errorf("csrf token mismatch")
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		return fmt.Errorf("missing authorization code")
	}

	tokens, err := exchangeCognitoCode(code)
	if err != nil {
		return fmt.Errorf("cognito token exchange failed: %w", err)
	}

	sessionToken, err := exchangeIDTokenForSession(tokens.IDToken)
	if err != nil {
		return fmt.Errorf("session token exchange failed: %w", err)
	}

	if err := keyring.Set(keyringService, keyringUser, sessionToken); err != nil {
		return fmt.Errorf("failed to store session token: %w", err)
	}

	return nil
}

func exchangeCognitoCode(code string) (*cognitoTokenResponse, error) {
	params := url.Values{
		"grant_type":   {"authorization_code"},
		"client_id":    {cognitoClientID},
		"code":         {code},
		"redirect_uri": {cognitoRedirectURI},
	}
	resp, err := http.PostForm(cognitoTokenURL, params)
	if err != nil {
		return nil, fmt.Errorf("failed to request tokens: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokens cognitoTokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokens, nil
}

func exchangeIDTokenForSession(idToken string) (string, error) {
	req, err := http.NewRequest("POST", apiTokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+idToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request session token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp apiTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse session token response: %w", err)
	}

	return tokenResp.Token, nil
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of your account",
	Long:  `Logs out of your current session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := keyring.Delete(keyringService, keyringUser); err != nil {
			fmt.Println("mobilecli is not logged in")
			return nil
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
			return fmt.Errorf("no oauth token found for mobilecli")
		}

		fmt.Println(token)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authTokenCmd)
}
