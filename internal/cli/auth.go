package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jamierumbelow/letterhead/internal/auth"
	"github.com/jamierumbelow/letterhead/internal/config"
	"github.com/jamierumbelow/letterhead/internal/gmail"
	"github.com/jamierumbelow/letterhead/pkg/types"
	"github.com/spf13/cobra"
)

// verifyAuthorisedAccount confirms the freshly authenticated Gmail account
// matches the configured account email. On mismatch the just-saved token is
// removed so the user can retry without manually clearing files.
func verifyAuthorisedAccount(ctx context.Context, configuredEmail string) error {
	result, err := auth.GetClient(ctx, configuredEmail)
	if err != nil {
		return fmt.Errorf("verify account: %w", err)
	}

	client, err := gmail.NewClient(ctx, result.Client)
	if err != nil {
		return fmt.Errorf("verify account: %w", err)
	}

	profile, err := client.GetProfile(ctx)
	if err != nil {
		return fmt.Errorf("verify account: %w", err)
	}

	gotEmail := strings.ToLower(strings.TrimSpace(profile.Email))
	wantEmail := strings.ToLower(strings.TrimSpace(configuredEmail))
	if gotEmail == wantEmail {
		return nil
	}

	if tokenPath, perr := config.TokenPath(configuredEmail); perr == nil {
		_ = os.Remove(tokenPath)
	}

	return fmt.Errorf("authorised %s in browser but configured account is %s; pick the matching Google account and re-run", profile.Email, configuredEmail)
}

func newAuthCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with Gmail",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := ensureInitialized()
			if err != nil {
				return err
			}

			acct, err := resolveAccount(cmd, cfg)
			if err != nil {
				return err
			}

			_, formatter, err := formatterFromCommand(cmd)
			if err != nil {
				return err
			}

			output := types.AuthOutput{
				Account: acct.Email,
				Method:  string(acct.AuthMethod),
			}

			if !force && auth.IsAuthenticated(acct.Email) {
				output.Authenticated = true
				return formatter.WriteAuth(cmd.OutOrStdout(), output)
			}

			ctx := context.Background()

			if force {
				oc, err := auth.LoadOAuthConfig(acct.Email)
				if err != nil {
					return NewExitErrorWithHint(ExitAuth, "letterhead auth", "authentication failed: %v", err)
				}
				if _, err := oc.Authenticate(ctx); err != nil {
					return NewExitErrorWithHint(ExitAuth, "letterhead auth", "authentication failed: %v", err)
				}
			} else {
				if _, err := auth.GetClient(ctx, acct.Email); err != nil {
					return NewExitErrorWithHint(ExitAuth, "letterhead auth", "authentication failed: %v", err)
				}
			}

			if acct.AuthMethod == config.AuthMethodOAuth {
				if err := verifyAuthorisedAccount(ctx, acct.Email); err != nil {
					return NewExitErrorWithHint(ExitAuth,
						fmt.Sprintf("letterhead auth --force --account %s", acct.Email),
						"%v", err)
				}
			}

			output.Authenticated = true
			return formatter.WriteAuth(cmd.OutOrStdout(), output)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force interactive re-authentication, replacing any stored token")

	return cmd
}
