package app

import (
	"encoding/json"
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/commands"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func NewAppCommand(p *commands.Params) *cobra.Command {
	debugCmd := &cobra.Command{
		Use:     "debug",
		Short:   "Print config",
		Example: "fyve debug",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			BindAppFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			appConfig, err := config.LoadAppConfig()
			if err != nil {
				return err
			}

			appConfigJson, _ := json.MarshalIndent(appConfig, "", "  ")
			fmt.Println(string(appConfigJson))

			_, err = p.NewServingClient("default")
			if err != nil {
				return err
			}

			return nil
		},
	}

	SetAppFlags(debugCmd.Flags())

	return debugCmd
}

func SetAppFlags(flags *flag.FlagSet) {
	flags.String("name", "", "App name.")
	flags.String("image", "", "Image to deploy. if not specified, the image will be built from the current directory.")
	flags.String("scale-down-delay", "15m", "keep containers around for a duration to avoid a cold star")
	flags.Int32("port", 3000, "Port to expose the application on (default: 3000)")
	flags.String("region", config.DefaultRegion, "AWS region")
}

func BindAppFlags(flags *flag.FlagSet) {
	_ = viper.BindPFlag("app", flags.Lookup("name"))
	_ = viper.BindPFlag("image", flags.Lookup("image"))
	_ = viper.BindPFlag("port", flags.Lookup("port"))
	_ = viper.BindPFlag("region", flags.Lookup("region"))
	_ = viper.BindPFlag("autoscaling.scaledown_delay", flags.Lookup("scale-down-delay"))
}
