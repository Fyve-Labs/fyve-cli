package commands

import (
	"context"
	"errors"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"time"
)

const (
	oidcRedirectURL = "http://localhost:8085/callback"
)

// NewLoginCommand creates a new login command
func NewLoginCommand() *cobra.Command {
	var (
		oidcIssuerURL string
		oidcClientID  string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Fyve App Platform",
		RunE: func(cmd *cobra.Command, args []string) error {
			oidcProvider, err := oidc.NewProvider(cmd.Context(), oidcIssuerURL)
			if err != nil {
				return err
			}

			// Create oauth2 config
			oauth2Config := &oauth2.Config{
				ClientID:    oidcClientID,
				RedirectURL: oidcRedirectURL,
				Endpoint:    oidcProvider.Endpoint(),
				Scopes:      []string{"openid", "profile", "email"},
			}

			// Generate random state for CSRF protection
			state := fmt.Sprintf("fyve-%d", time.Now().Unix())

			// Create channel to receive auth code
			codeChan := make(chan string, 1)
			errChan := make(chan error, 1)

			// Start HTTP server to handle callback
			server := &http.Server{Addr: ":8085"}

			// Create a context for server shutdown
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			// Set up http handler for the callback
			http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
				// Verify state parameter to prevent CSRF
				if r.URL.Query().Get("state") != state {
					errChan <- errors.New("invalid state parameter")
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprintf(w, "Error: State mismatch. Authentication failed.")
					return
				}

				// Get authorization code
				code := r.URL.Query().Get("code")
				if code == "" {
					errChan <- errors.New("no code in callback response")
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprintf(w, "Error: No authorization code received.")
					return
				}

				// Send the code to the main goroutine
				codeChan <- code

				// Show success page
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "Login successful! You can close this window and return to the CLI.")

				// Shutdown the server after a short delay
				go func() {
					time.Sleep(1 * time.Second)
					server.Shutdown(context.Background())
				}()
			})

			// Start the server in a goroutine
			go func() {
				if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
					errChan <- err
				}
			}()

			// Generate the auth URL and open it in the browser
			authURL := oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
			fmt.Fprintln(cmd.OutOrStdout(), "Opening browser for login...")
			if err := browser.OpenURL(authURL); err != nil {
				fmt.Fprintln(os.Stderr, "failed to open browser")
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Waiting for authentication...")

			// Wait for code or error
			var code string
			select {
			case code = <-codeChan:
				// Received code, continue with token exchange
			case err := <-errChan:
				return err
			case <-ctx.Done():
				return errors.New("authentication timed out")
			}

			// Exchange the code for a token
			token, err := oauth2Config.Exchange(ctx, code)
			if err != nil {
				return err
			}

			idToken, ok := token.Extra("id_token").(string)
			if !ok {
				return errors.New("no id_token found in oauth2 token")
			}

			// Create auth config
			authConfig := config.AuthConfig{
				IDToken:      idToken,
				AccessToken:  token.AccessToken,
				RefreshToken: token.RefreshToken,
				Expiry:       token.Expiry,
			}

			// Save auth config to ~/.fyve/config.json
			if err := config.SaveAuthConfig(authConfig); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logged in\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&oidcIssuerURL, "issuer", "https://auth.fyve.dev", "OIDC issuer URL")
	cmd.Flags().StringVar(&oidcClientID, "client-id", "fyve-k8s", "OIDC client ID")

	return cmd
}
