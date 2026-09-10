package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/utils"
)

// OAuth authorization code + PKCE against the server's built-in "mobilecli" client
// (RFC 8252 native app: ephemeral loopback port, no client secret).
const (
	oauthClientID     = "mobilecli"
	oauthAuthorizeURL = "https://app.mobilenext.ai/login/oauth/authorize"
	// #nosec G101 -- this is the public token endpoint URL, not a credential
	oauthTokenURL     = "https://app.mobilenext.ai/login/oauth/token"
	oauthCallbackPath = "/callback"
	// Longer than the server's 10-minute consent session: the CLI must still be listening for
	// as long as the browser can legitimately redirect back.
	oauthLoginTimeout = 15 * time.Minute
)

type oauthCallback struct {
	code  string
	state string
	err   string
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error,omitempty"`
}

func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkceChallenge is S256 per RFC 7636 §4.2.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// detectAgent names the coding agent driving mobilecli, if any, so the server can tell
// "a human in a terminal" from "Claude Code". Empty when none is recognised.
func detectAgent(getenv func(string) string) string {
	if getenv("CLAUDECODE") != "" {
		return "claude-code"
	}
	return ""
}

func buildAuthorizeURL(authorizeURL, redirectURI, challenge, state, agent string) string {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {oauthClientID},
		"redirect_uri":          {redirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}
	if agent != "" {
		q.Set("agent", agent)
	}
	return authorizeURL + "?" + q.Encode()
}

// waitForCallback serves one request on listener and returns what the browser brought back.
func waitForCallback(ctx context.Context, listener net.Listener) (oauthCallback, error) {
	got := make(chan oauthCallback, 1)
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != oauthCallbackPath {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		cb := oauthCallback{code: q.Get("code"), state: q.Get("state"), err: q.Get("error")}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page := "<h2>Logged in to mobilecli</h2><p>You can close this tab.</p>"
		if cb.err != "" || cb.code == "" {
			page = "<h2>Login failed</h2><p>You can close this tab and retry in the terminal.</p>"
		}
		// the login itself already succeeded or failed by now, so a browser that
		// hung up before reading the page changes nothing
		if _, writeErr := fmt.Fprint(w, page); writeErr != nil {
			utils.Verbose("failed to write callback page: %v", writeErr)
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case got <- cb:
		default:
		}
	})
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			utils.Verbose("callback listener stopped: %v", serveErr)
		}
	}()
	// Shutdown, not Close: it waits for the in-flight handler to finish writing the page, so the
	// browser never sees the connection drop mid-response.
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			utils.Verbose("callback listener shutdown: %v", shutdownErr)
		}
	}()

	select {
	case cb := <-got:
		return cb, nil
	case <-ctx.Done():
		return oauthCallback{}, fmt.Errorf("no browser login within %s, run `mobilecli auth login` again", oauthLoginTimeout)
	}
}

func exchangeCode(tokenURL, code, verifier, redirectURI string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {oauthClientID},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", utils.UserAgent())
	resp, err := authHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			utils.Verbose("failed to close token response: %v", closeErr)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}
	var tok oauthTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}
	if tok.Error != "" || tok.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}
	return tok.AccessToken, nil
}

// runOAuthLogin opens the browser to the consent screen (pick organization, approve) and
// receives the code on a loopback port. Returns the access token.
func runOAuthLogin(authorizeURL, tokenURL string) (string, error) {
	verifier, err := randomURLSafe(32)
	if err != nil {
		return "", err
	}
	state, err := randomURLSafe(16)
	if err != nil {
		return "", err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to listen for browser callback: %w", err)
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil {
			utils.Verbose("failed to close callback listener: %v", closeErr)
		}
	}()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return "", fmt.Errorf("callback listener is not tcp: %T", listener.Addr())
	}
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d%s", addr.Port, oauthCallbackPath)

	loginURL := buildAuthorizeURL(authorizeURL, redirectURI, pkceChallenge(verifier), state, detectAgent(os.Getenv))
	// A missing xdg-open (minimal Linux, WSL) is not fatal: the loopback callback works just as
	// well when the user pastes the URL themselves.
	if err := openBrowser(loginURL); err != nil {
		fmt.Printf("Could not open a browser (%v). Open this URL to log in:\n\n\t%s\n\n", err, loginURL)
	} else {
		fmt.Printf("Opened your browser to log in. If it did not open, visit:\n\n\t%s\n\n", loginURL)
	}
	fmt.Println("Waiting for authorization...")

	ctx, cancel := context.WithTimeout(context.Background(), oauthLoginTimeout)
	defer cancel()
	cb, err := waitForCallback(ctx, listener)
	if err != nil {
		return "", err
	}
	if cb.err != "" {
		return "", fmt.Errorf("login denied: %s", cb.err)
	}
	if cb.state != state {
		return "", errors.New("login callback state mismatch")
	}
	if cb.code == "" {
		return "", errors.New("login callback carried no authorization code, run `mobilecli auth login` again")
	}
	return exchangeCode(tokenURL, cb.code, verifier, redirectURI)
}
