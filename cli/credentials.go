package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

// The auth token is normally kept in the OS keyring (macOS Keychain, Windows
// Credential Manager, freedesktop Secret Service on Linux). Headless hosts (CI,
// Docker, SSH-only boxes, Checkly) usually have no keyring backend, so on those
// the user can pass --insecure-storage to store the token in a plaintext 0600
// file under the user config dir instead. The flag is all-or-nothing: when set,
// the keyring is skipped entirely (read, write, and delete) on every platform.

// insecureStorage is bound to the global --insecure-storage flag.
var insecureStorage bool

// credentialsFilePath returns the path to the plaintext token file used when
// --insecure-storage is set: $XDG_CONFIG_HOME/mobilecli/credentials, falling
// back to ~/.config/mobilecli/credentials. We deliberately use ~/.config on
// every platform (rather than os.UserConfigDir, which is ~/Library on macOS) so
// the location is identical everywhere, matching how the GitHub CLI behaves.
func credentialsFilePath() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "mobilecli", "credentials"), nil
}

// storeToken saves the token in the keyring, or in the credentials file when
// --insecure-storage is set.
func storeToken(token string) error {
	if insecureStorage {
		return storeTokenInFile(token)
	}
	if err := keyring.Set(keyringService, keyringUser, token); err != nil {
		return fmt.Errorf("failed to store token in keyring: %w (on a headless host, re-run with --insecure-storage)", err)
	}
	return nil
}

func storeTokenInFile(token string) error {
	path, err := credentialsFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create credentials dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}
	return nil
}

// loadToken returns the token from, in order: the MOBILECLI_TOKEN env var, then
// either the credentials file (with --insecure-storage) or the OS keyring. It
// returns keyring.ErrNotFound when no token is found, so callers can detect
// "not logged in".
func loadToken() (string, error) {
	if token := os.Getenv("MOBILECLI_TOKEN"); token != "" {
		return token, nil
	}

	if insecureStorage {
		return loadTokenFromFile()
	}

	token, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", keyring.ErrNotFound
		}
		return "", fmt.Errorf("failed to get token from keyring: %w", err)
	}
	return token, nil
}

// tokenLoadTimeout bounds how long loading a token from the OS keyring may
// block. Some keyring backends never return - notably the freedesktop Secret
// Service when its login collection is locked and no interactive prompter is
// available (e.g. commands auto-started from an SSH session). Without this
// bound, every command that touches the keyring would hang forever.
const tokenLoadTimeout = 3 * time.Second

var (
	tokenLoadOnce sync.Once
	tokenLoadVal  string
	tokenLoadErr  error
)

// loadTokenWithTimeout is loadToken with a hard deadline, memoized so the
// timeout is paid at most once per process. Callers that must not hang, like
// the root pre-run and daemon startup, use this instead of loadToken.
func loadTokenWithTimeout() (string, error) {
	tokenLoadOnce.Do(func() {
		tokenLoadVal, tokenLoadErr = loadTokenDeadline(loadToken, tokenLoadTimeout)
	})
	return tokenLoadVal, tokenLoadErr
}

// loadTokenDeadline runs fn but gives up after timeout. It is split out from
// loadTokenWithTimeout so the deadline behavior can be unit-tested.
func loadTokenDeadline(fn func() (string, error), timeout time.Duration) (string, error) {
	type result struct {
		token string
		err   error
	}

	ch := make(chan result, 1)
	go func() {
		token, err := fn()
		ch <- result{token: token, err: err}
	}()

	select {
	case r := <-ch:
		return r.token, r.err
	case <-time.After(timeout):
		return "", fmt.Errorf("timed out after %s reading credentials; the OS keyring may be locked (unlock it, or use --insecure-storage)", timeout)
	}
}

func loadTokenFromFile() (string, error) {
	path, err := credentialsFilePath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is the fixed credentials file under the user config dir
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", keyring.ErrNotFound
		}
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", keyring.ErrNotFound
	}
	return token, nil
}

// deleteToken removes the stored token. It returns keyring.ErrNotFound when no
// token was stored, so logout can report "not logged in".
func deleteToken() error {
	if insecureStorage {
		path, err := credentialsFilePath()
		if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return keyring.ErrNotFound
			}
			return err
		}
		return nil
	}
	return keyring.Delete(keyringService, keyringUser)
}
