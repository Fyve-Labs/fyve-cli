package app

import (
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/commands"
	"github.com/spf13/cobra"
	"strings"
	"text/tabwriter"
	"time"
)

// NewListCommand returns a new command for listing all applications
func NewListCommand(p *commands.Params) *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all deployed applications",
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

			// Create v1beta1 client to access DomainMappings
			v1beta1Client, err := p.NewServingV1beta1Client(namespace)
			if err != nil {
				return err
			}

			// Get DomainMappings for the namespace
			domainMappingList, err := v1beta1Client.ListDomainMappings(cmd.Context())
			if err != nil {
				return err
			}

			// Create a map of service name to domain mappings
			serviceDomains := make(map[string][]string)
			for _, obj := range domainMappingList.Items {
				url := obj.Status.Address.URL.String()
				if url != "" {
					refName := obj.Spec.Ref.Name
					serviceDomains[refName] = append(serviceDomains[refName], url)
				}
			}

			// Set up tabwriter for clean formatting
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			defer w.Flush()

			// Print headers
			fmt.Fprintln(w, "NAME\tURL\tPRODUCTION URL\tREADY\tGENERATION\tAGE")

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

				// Get production URLs from domain mappings for this service
				productionURLs := "<none>"
				if urlList, exists := serviceDomains[service.Name]; exists && len(urlList) > 0 {
					productionURLs = strings.Join(urlList, ", ")
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
					service.Name,
					url,
					productionURLs,
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
