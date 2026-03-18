package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestLoadOAuthConfigRequiresAccountEmail(t *testing.T) {
	t.Parallel()

	_, err := LoadOAuthConfig("")
	if err != ErrAccountRequired {
		t.Fatalf("LoadOAuthConfig('') error = %v, want ErrAccountRequired", err)
	}

	_, err = LoadOAuthConfig("   ")
	if err != ErrAccountRequired {
		t.Fatalf("LoadOAuthConfig('   ') error = %v, want ErrAccountRequired", err)
	}
}

func TestLoadOAuthConfigFromEnv(t *testing.T) {
	// Don't run in parallel — modifies env
	t.Setenv("LETTERHEAD_CLIENT_ID", "test-client-id")
	t.Setenv("LETTERHEAD_CLIENT_SECRET", "test-client-secret")
	// Point XDG_CONFIG_HOME to a temp dir so credentials.json is not found
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	oc, err := LoadOAuthConfig("user@example.com")
	if err != nil {
		t.Fatalf("LoadOAuthConfig() error = %v", err)
	}

	if oc.oauth2Config.ClientID != "test-client-id" {
		t.Errorf("ClientID = %q, want %q", oc.oauth2Config.ClientID, "test-client-id")
	}
	if oc.oauth2Config.ClientSecret != "test-client-secret" {
		t.Errorf("ClientSecret = %q, want %q", oc.oauth2Config.ClientSecret, "test-client-secret")
	}
	if oc.accountEmail != "user@example.com" {
		t.Errorf("accountEmail = %q, want %q", oc.accountEmail, "user@example.com")
	}
}

func TestLoadOAuthConfigNoCredentials(t *testing.T) {
	t.Setenv("LETTERHEAD_CLIENT_ID", "")
	t.Setenv("LETTERHEAD_CLIENT_SECRET", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := LoadOAuthConfig("user@example.com")
	if err != ErrNoCredentials {
		t.Fatalf("LoadOAuthConfig() error = %v, want ErrNoCredentials", err)
	}
}

func TestLoadOAuthConfigFromCredentialsFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("LETTERHEAD_CLIENT_ID", "")
	t.Setenv("LETTERHEAD_CLIENT_SECRET", "")

	credDir := filepath.Join(tmpDir, "letterhead")
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	// Write a minimal installed-app credentials.json
	creds := map[string]any{
		"installed": map[string]any{
			"client_id":     "file-client-id",
			"client_secret": "file-client-secret",
			"auth_uri":      "https://accounts.google.com/o/oauth2/auth",
			"token_uri":     "https://oauth2.googleapis.com/token",
			"redirect_uris": []string{"urn:ietf:wg:oauth:2.0:oob"},
		},
	}
	data, _ := json.Marshal(creds)
	if err := os.WriteFile(filepath.Join(credDir, "credentials.json"), data, 0o600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	oc, err := LoadOAuthConfig("user@example.com")
	if err != nil {
		t.Fatalf("LoadOAuthConfig() error = %v", err)
	}

	if oc.oauth2Config.ClientID != "file-client-id" {
		t.Errorf("ClientID = %q, want %q", oc.oauth2Config.ClientID, "file-client-id")
	}
}

func TestTokenSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("LETTERHEAD_CLIENT_ID", "test-id")
	t.Setenv("LETTERHEAD_CLIENT_SECRET", "test-secret")

	oc, err := LoadOAuthConfig("user@example.com")
	if err != nil {
		t.Fatalf("LoadOAuthConfig() error = %v", err)
	}

	token := &oauth2.Token{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}

	if err := oc.saveToken(token); err != nil {
		t.Fatalf("saveToken() error = %v", err)
	}

	loaded, err := oc.loadToken()
	if err != nil {
		t.Fatalf("loadToken() error = %v", err)
	}

	if loaded.AccessToken != token.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, token.AccessToken)
	}
	if loaded.RefreshToken != token.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, token.RefreshToken)
	}
}

func TestHasToken(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("LETTERHEAD_CLIENT_ID", "test-id")
	t.Setenv("LETTERHEAD_CLIENT_SECRET", "test-secret")

	oc, err := LoadOAuthConfig("user@example.com")
	if err != nil {
		t.Fatalf("LoadOAuthConfig() error = %v", err)
	}

	if oc.HasToken() {
		t.Fatalf("HasToken() = true before save")
	}

	token := &oauth2.Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	if err := oc.saveToken(token); err != nil {
		t.Fatalf("saveToken() error = %v", err)
	}

	if !oc.HasToken() {
		t.Fatalf("HasToken() = false after save")
	}
}

func TestTokenFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("LETTERHEAD_CLIENT_ID", "test-id")
	t.Setenv("LETTERHEAD_CLIENT_SECRET", "test-secret")

	oc, err := LoadOAuthConfig("user@example.com")
	if err != nil {
		t.Fatalf("LoadOAuthConfig() error = %v", err)
	}

	token := &oauth2.Token{
		AccessToken: "access",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}
	if err := oc.saveToken(token); err != nil {
		t.Fatalf("saveToken() error = %v", err)
	}

	// Verify file permissions are 0600
	tokenPath, _ := tokenPathForAccount(oc.accountEmail)
	info, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("token file permissions = %o, want 0600", perm)
	}
}

func tokenPathForAccount(email string) (string, error) {
	// Reuse the config package's TokenPath
	return configTokenPath(email)
}

var configTokenPath = func(email string) (string, error) {
	// Import cycle avoidance: we call config.TokenPath at runtime
	// but for tests we can use the actual function
	return loadTokenPathFromConfig(email)
}

func loadTokenPathFromConfig(email string) (string, error) {
	// This uses the same logic as config.TokenPath
	// For test purposes, we just check the file exists in XDG_CONFIG_HOME
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}

	// Find the token file (there should be exactly one)
	matches, err := filepath.Glob(filepath.Join(configHome, "letterhead", "token_*.json"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", os.ErrNotExist
	}
	return matches[0], nil
}
