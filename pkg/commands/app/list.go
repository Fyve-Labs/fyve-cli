package app

import (
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/commands"
	"github.com/spf13/cobra"
	"text/tabwriter"
	"time"
)

// NewListCommand returns a new command for listing all Knative serving applications
func NewListCommand(p *commands.Params) *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all deployed Knative serving applications",
		Example: "fyve list",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create a Serving client
			client, err := p.NewServingClient(namespace)
			if err != nil {
				return err
			}

			// Get the list of services
			serviceList, err := client.ListServices(cmd.Context())
			if err != nil {
				return err
			}

			// Set up tabwriter for clean formatting
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			defer w.Flush()

			// Print headers
			fmt.Fprintln(w, "NAME\tURL\tREADY\tGENERATION\tAGE")

			if len(serviceList.Items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No applications found.")
				return nil
			}

			// Print each service
			for _, service := range serviceList.Items {
				age := formatAge(service.CreationTimestamp.Time)
				ready := "Unknown"
				if len(service.Status.Conditions) > 0 {
					for _, cond := range service.Status.Conditions {
						if cond.Type == "Ready" {
							ready = string(cond.Status)
							break
						}
					}
				}

				url := service.Status.URL.String()
				if url == "" {
					url = "<none>"
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
					service.Name,
					url,
					ready,
					service.Generation,
					age,
				)
			}

			return nil
		},
	}

	// Add namespace flag
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to list applications from (default is current namespace)")

	return cmd
}

// formatAge returns a human-readable string representing the time since the given timestamp
func formatAge(timestamp time.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}
	return formatDuration(time.Since(timestamp))
}

// formatDuration formats a duration in a concise form (e.g., "5d", "2h", "10m")
func formatDuration(d time.Duration) string {
	// Format duration in a concise form
	seconds := int(d.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}
	days := hours / 24
	return fmt.Sprintf("%dd", days)
}
