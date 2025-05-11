package root

import (
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/commands"
	"github.com/fyve-labs/fyve-cli/pkg/commands/app"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"github.com/spf13/cobra"
	"knative.dev/client/pkg/flags"
	"os"
	"path/filepath"
)

func NewRootCommand() (*cobra.Command, error) {
	p := &commands.FyveParams{}
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

	rootCmd.AddCommand(commands.NewDeployCmd())
	rootCmd.AddCommand(app.NewPublishCommand(p))
	rootCmd.AddCommand(app.NewUnPublishCommand(p))
	rootCmd.AddCommand(commands.NewUpdateCmd())
	rootCmd.AddCommand(commands.NewSocketProxyCmd())

	return rootCmd, nil
}

func GetBinaryName() string {
	_, name := filepath.Split(os.Args[0])
	return name
}
