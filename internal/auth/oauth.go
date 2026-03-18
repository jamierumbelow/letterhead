package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jamierumbelow/letterhead/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const gmailReadOnlyScope = "https://www.googleapis.com/auth/gmail.readonly"

// Bundled OAuth client credentials for the letterhead desktop app.
// For installed/desktop apps the client secret is not truly secret
// (see https://developers.google.com/identity/protocols/oauth2/native-app).
// Users can override these by placing a credentials.json in the config dir
// or setting LETTERHEAD_CLIENT_ID / LETTERHEAD_CLIENT_SECRET env vars.
var (
	bundledClientID     = "" // set via -ldflags at build time
	bundledClientSecret = "" // set via -ldflags at build time
)

var (
	ErrNoCredentials   = errors.New("no OAuth credentials found; set LETTERHEAD_CLIENT_ID and LETTERHEAD_CLIENT_SECRET, or place a credentials.json in ~/.config/letterhead/")
	ErrAccountRequired = errors.New("account email is required for token storage")
)

// OAuthConfig holds the resolved OAuth2 configuration.
type OAuthConfig struct {
	oauth2Config *oauth2.Config
	accountEmail string
}

// LoadOAuthConfig resolves OAuth2 credentials from (in order):
//  1. A credentials.json file in the config directory (user-supplied Google Cloud project)
//  2. Environment variables LETTERHEAD_CLIENT_ID and LETTERHEAD_CLIENT_SECRET
//  3. Bundled client credentials compiled into the binary
func LoadOAuthConfig(accountEmail string) (*OAuthConfig, error) {
	accountEmail = strings.TrimSpace(strings.ToLower(accountEmail))
	if accountEmail == "" {
		return nil, ErrAccountRequired
	}

	cfg, err := loadFromCredentialsFile()
	if err == nil {
		return &OAuthConfig{oauth2Config: cfg, accountEmail: accountEmail}, nil
	}

	cfg, err = loadFromEnv()
	if err == nil {
		return &OAuthConfig{oauth2Config: cfg, accountEmail: accountEmail}, nil
	}

	cfg, err = loadBundled()
	if err == nil {
		return &OAuthConfig{oauth2Config: cfg, accountEmail: accountEmail}, nil
	}

	return nil, ErrNoCredentials
}

// Authenticate runs the full OAuth2 installed-app flow:
// starts a local redirect listener, opens the browser, exchanges
// the code for tokens, and persists them.
func (oc *OAuthConfig) Authenticate(ctx context.Context) (*oauth2.Token, error) {
	// Try local redirect first
	token, err := oc.authenticateWithRedirect(ctx)
	if err == nil {
		if err := oc.saveToken(token); err != nil {
			return nil, fmt.Errorf("save token: %w", err)
		}
		return token, nil
	}

	// Fall back to manual paste-back
	token, err = oc.authenticateManual(ctx)
	if err != nil {
		return nil, err
	}

	if err := oc.saveToken(token); err != nil {
		return nil, fmt.Errorf("save token: %w", err)
	}

	return token, nil
}

// GetAuthenticatedClient returns an http.Client with a valid token.
// It loads the persisted token, refreshing if needed.
func (oc *OAuthConfig) GetAuthenticatedClient(ctx context.Context) (*http.Client, error) {
	token, err := oc.loadToken()
	if err != nil {
		return nil, fmt.Errorf("no stored token (run letterhead init first): %w", err)
	}

	// oauth2 TokenSource handles refresh automatically
	return oc.oauth2Config.Client(ctx, token), nil
}

// HasToken returns true if a persisted token file exists for this account.
func (oc *OAuthConfig) HasToken() bool {
	tokenPath, err := config.TokenPath(oc.accountEmail)
	if err != nil {
		return false
	}
	_, err = os.Stat(tokenPath)
	return err == nil
}

func (oc *OAuthConfig) authenticateWithRedirect(ctx context.Context) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	cfg := *oc.oauth2Config
	cfg.RedirectURL = redirectURL

	state := "letterhead-auth"
	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- errors.New("state mismatch in OAuth callback")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}

		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errCh <- fmt.Errorf("OAuth error: %s", errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- errors.New("no code in OAuth callback")
			http.Error(w, "no code", http.StatusBadRequest)
			return
		}

		fmt.Fprintln(w, "Authorization complete. You can close this tab.")
		codeCh <- code
	})

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	defer server.Close()

	if err := openBrowser(authURL); err != nil {
		return nil, fmt.Errorf("could not open browser: %w", err)
	}

	select {
	case code := <-codeCh:
		return cfg.Exchange(ctx, code)
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (oc *OAuthConfig) authenticateManual(ctx context.Context) (*oauth2.Token, error) {
	cfg := *oc.oauth2Config
	cfg.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"

	authURL := cfg.AuthCodeURL("letterhead-auth", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	fmt.Println("Open this URL in your browser to authorize letterhead:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	fmt.Print("Paste the authorization code here: ")

	var code string
	if _, err := fmt.Scanln(&code); err != nil {
		return nil, fmt.Errorf("read code: %w", err)
	}

	return cfg.Exchange(ctx, strings.TrimSpace(code))
}

func (oc *OAuthConfig) saveToken(token *oauth2.Token) error {
	tokenPath, err := config.TokenPath(oc.accountEmail)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		return err
	}

	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	return os.WriteFile(tokenPath, data, 0o600)
}

func (oc *OAuthConfig) loadToken() (*oauth2.Token, error) {
	tokenPath, err := config.TokenPath(oc.accountEmail)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

func loadFromCredentialsFile() (*oauth2.Config, error) {
	configHome, err := configDir()
	if err != nil {
		return nil, err
	}

	credPath := filepath.Join(configHome, "letterhead", "credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return nil, err
	}

	cfg, err := google.ConfigFromJSON(data, gmailReadOnlyScope)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadFromEnv() (*oauth2.Config, error) {
	clientID := os.Getenv("LETTERHEAD_CLIENT_ID")
	clientSecret := os.Getenv("LETTERHEAD_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return nil, ErrNoCredentials
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{gmailReadOnlyScope},
	}, nil
}

func loadBundled() (*oauth2.Config, error) {
	if bundledClientID == "" || bundledClientSecret == "" {
		return nil, ErrNoCredentials
	}

	return &oauth2.Config{
		ClientID:     bundledClientID,
		ClientSecret: bundledClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{gmailReadOnlyScope},
	}, nil
}

func configDir() (string, error) {
	if value := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
