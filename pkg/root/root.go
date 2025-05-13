package root

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/fyve-labs/fyve-cli/pkg/commands"
	"github.com/fyve-labs/fyve-cli/pkg/commands/app"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"knative.dev/client/pkg/flags"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func NewRootCommand() (*cobra.Command, error) {
	p := &commands.Params{}
	p.Initialize()

	rootName := GetBinaryName()
	rootCmd := &cobra.Command{
		Use:   rootName,
		Short: fmt.Sprintf("%s manages FyveLabs applications", rootName),
		Long:  fmt.Sprintf(`%s is a CLI tool for deploying NextJS applications easier`, rootName),

		// Disable docs header
		DisableAutoGenTag: true,

		SilenceUsage:  true,
		SilenceErrors: true,

		// Validate our boolean configs
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return flags.ReconcileBoolFlags(cmd.Flags())
		},
	}

	// Bootstrap flags
	config.AddBootstrapFlags(rootCmd.PersistentFlags())

	// Global Kube' flags
	p.Params.SetFlags(rootCmd.PersistentFlags())

	rootCmd.AddCommand(commands.NewLoginCommand())
	AddKubeCommand(p, rootCmd, commands.NewDeployCmd(p))
	AddKubeCommand(p, rootCmd, app.NewPublishCommand(p))
	AddKubeCommand(p, rootCmd, app.NewUnPublishCommand(p))
	AddKubeCommand(p, rootCmd, app.NewListCommand(p))
	rootCmd.AddCommand(commands.NewUpdateCmd())
	rootCmd.AddCommand(commands.NewLoginCommand())
	rootCmd.AddCommand(commands.NewLogoutCommand())
	rootCmd.AddCommand(commands.NewSocketProxyCmd())

	return rootCmd, nil
}

func AddKubeCommand(p *commands.Params, root, cmd *cobra.Command) {
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// Force built in kubeconfig if not set
		if len(p.Params.KubeCfgPath) == 0 {
			// Auto exchange credential on GitHub actions
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Second*60)
			defer cancel()
			err := exchangeGithubCredential(ctx)
			if err != nil {
				return fmt.Errorf("exchange Github credential: %w", err)
			}

			kubeconfigPath, err := config.LoadKubeconfig()
			if err != nil {
				return err
			}

			p.Params.KubeCfgPath = kubeconfigPath
		}

		return nil
	}

	root.AddCommand(cmd)
}

func GetBinaryName() string {
	_, name := filepath.Split(os.Args[0])
	return name
}

func exchangeGithubCredential(ctx context.Context) error {
	var token, tokenURL string
	if token = os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN"); token == "" {
		return nil
	}

	if tokenURL = os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL"); tokenURL == "" {
		return nil
	}

	// request Github ID token
	client := &http.Client{}
	req, err := http.NewRequest("GET", tokenURL+"&audience=api://FyveTokenExchange", nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "bearer "+token)
	req.Header.Add("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var respData struct {
		Value string `json:"value"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return err
	}

	githubToken := respData.Value
	log.Printf("Github ID token: %s\n", githubToken)

	oidcIssuerURL := viper.GetString("oidc.issuer.url")
	oidcProvider, err := oidc.NewProvider(ctx, oidcIssuerURL)
	if err != nil {
		return err
	}

	exchangedToken, err := exchangeForDexToken(oidcProvider.Endpoint().TokenURL, githubToken, "fyve-cli", "public", "fyve-cluster")
	if err != nil {
		return err
	}

	log.Printf("Github token exchanged to Fyve token: %s\n", exchangedToken)
	return config.SaveAuthConfig(config.AuthConfig{
		AccessToken: exchangedToken,
	})
}

func exchangeForDexToken(tokenEndpoint, githubToken string, clientID, clientSecret, crossTrustClientId string) (string, error) {
	data := url.Values{}
	data.Set("connector_id", "github-actions")
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	data.Set("scope", fmt.Sprintf("openid groups federated:id audience:server:client_id:%s", crossTrustClientId))
	data.Set("requested_token_type", "urn:ietf:params:oauth:token-type:id_token")
	data.Set("subject_token", githubToken)
	data.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")

	client := &http.Client{}
	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp map[string]interface{}

	if err = json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	tokenRespString, _ := json.Marshal(tokenResp)
	log.Printf("Token exchange response: %s\n", tokenRespString)

	if val, ok := tokenResp["access_token"].(string); ok && val != "" {
		return val, nil
	}

	return "", errors.New("exchange failed")
}
