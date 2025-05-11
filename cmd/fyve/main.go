package main

import (
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"github.com/fyve-labs/fyve-cli/pkg/root"
	"os"
)

func main() {
	os.Exit(runWithExit())
}

func runWithExit() int {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}

func run() error {
	err := config.BootstrapConfig()
	if err != nil {
		return err
	}

	rootCmd, _ := root.NewRootCommand()
	return rootCmd.Execute()
}
