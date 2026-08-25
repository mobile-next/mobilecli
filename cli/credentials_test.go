package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

// usingInsecureStorage turns on the --insecure-storage behavior for one test and
// restores it afterwards.
func usingInsecureStorage(t *testing.T) {
	t.Helper()
	insecureStorage = true
	t.Cleanup(func() { insecureStorage = false })
}

// usingKeyringStorage forces the keyring path (the default) for clarity.
func usingKeyringStorage(t *testing.T) {
	t.Helper()
	insecureStorage = false
}

// failingKeyring makes every keyring call return an error, so a test can prove
// that --insecure-storage never touches the keyring.
func failingKeyring(t *testing.T) {
	t.Helper()
	keyring.MockInitWithError(errors.New("keyring must not be used"))
}

// workingKeyring provides an in-memory keyring backend.
func workingKeyring(t *testing.T) {
	t.Helper()
	keyring.MockInit()
}

// redirectConfigDir points os.UserConfigDir() at a throwaway temp dir so tests
// never touch the real credentials file.
func redirectConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
}

func clearTokenEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MOBILECLI_TOKEN", "")
}

func TestStoreAndLoadTokenUsesKeyringByDefault(t *testing.T) {
	usingKeyringStorage(t)
	workingKeyring(t)
	clearTokenEnv(t)

	if err := storeToken("tok-keyring"); err != nil {
		t.Fatalf("storeToken: %v", err)
	}

	got, err := loadToken()
	if err != nil {
		t.Fatalf("loadToken: %v", err)
	}
	if got != "tok-keyring" {
		t.Fatalf("loadToken = %q, want %q", got, "tok-keyring")
	}
}

func TestInsecureStorageUsesFileAndSkipsKeyring(t *testing.T) {
	usingInsecureStorage(t)
	failingKeyring(t) // proves the keyring is never touched in this mode
	redirectConfigDir(t)
	clearTokenEnv(t)

	if err := storeToken("tok-file"); err != nil {
		t.Fatalf("storeToken with --insecure-storage: %v", err)
	}

	path, _ := credentialsFilePath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("credentials file was not written: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("credentials file mode = %o, want 600", perm)
	}

	got, err := loadToken()
	if err != nil {
		t.Fatalf("loadToken from file: %v", err)
	}
	if got != "tok-file" {
		t.Fatalf("loadToken = %q, want %q", got, "tok-file")
	}
}

func TestKeyringStorageDoesNotWriteFile(t *testing.T) {
	usingKeyringStorage(t)
	workingKeyring(t)
	redirectConfigDir(t)
	clearTokenEnv(t)

	if err := storeToken("tok"); err != nil {
		t.Fatalf("storeToken: %v", err)
	}

	path, _ := credentialsFilePath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("no credentials file should exist without --insecure-storage, stat err = %v", err)
	}
}

func TestLoadTokenPrefersEnvVar(t *testing.T) {
	usingInsecureStorage(t)
	failingKeyring(t)
	redirectConfigDir(t)
	t.Setenv("MOBILECLI_TOKEN", "tok-env")

	got, err := loadToken()
	if err != nil {
		t.Fatalf("loadToken: %v", err)
	}
	if got != "tok-env" {
		t.Fatalf("loadToken = %q, want %q", got, "tok-env")
	}
}

func TestLoadTokenReturnsNotFoundFromFile(t *testing.T) {
	usingInsecureStorage(t)
	redirectConfigDir(t)
	clearTokenEnv(t)

	if _, err := loadToken(); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("loadToken err = %v, want keyring.ErrNotFound", err)
	}
}

func TestLoadTokenReturnsNotFoundFromEmptyKeyring(t *testing.T) {
	usingKeyringStorage(t)
	workingKeyring(t)
	clearTokenEnv(t)

	if _, err := loadToken(); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("loadToken err = %v, want keyring.ErrNotFound", err)
	}
}

func TestDeleteTokenRemovesFile(t *testing.T) {
	usingInsecureStorage(t)
	redirectConfigDir(t)
	clearTokenEnv(t)

	if err := storeToken("tok"); err != nil {
		t.Fatalf("storeToken: %v", err)
	}
	if err := deleteToken(); err != nil {
		t.Fatalf("deleteToken: %v", err)
	}

	path, _ := credentialsFilePath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("credentials file should be gone, stat err = %v", err)
	}

	if err := deleteToken(); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("second deleteToken err = %v, want keyring.ErrNotFound", err)
	}
}
