package fyve

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "fyve",
	Short: "fyve is a CLI tool for deploying NextJS applications",
	Long: `fyve is a CLI tool for deploying NextJS applications to a Docker host.
It supports building, pushing to ECR, and deploying to a Docker host.`,
}

// Execute executes the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
