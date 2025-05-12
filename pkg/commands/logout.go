package commands

import (
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

// NewLogoutCommand creates a new logout command
func NewLogoutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clears the local auth config and logs out of the Fyve App Platform.",
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			fyveDir := filepath.Join(homeDir, ".fyve")
			return os.RemoveAll(fyveDir)
		},
	}

	return cmd
}
