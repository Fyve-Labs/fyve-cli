package main

import (
	"fmt"
	"os"

	"github.com/fyve-labs/fyve-cli/cmd/fyve"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "fyve",
		Short: "Fyve CLI for building and deploying NextJS applications",
		Long: `Fyve CLI is a tool for building and deploying NextJS applications 
to a remote docker host. It handles the entire process from building 
Docker images to deploying them on your infrastructure.`,
	}

	// Add commands
	rootCmd.AddCommand(fyve.DeployCmd())

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
