package main

import (
	"fmt"
	"github.com/spf13/pflag"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ivanklee86/tangle/internal/cli"
)

var (
	// Build information (injected by goreleaser).
	version = "dev"
)

const (
	defaultConfigFilename = "tangle"
	envPrefix             = "TANGLE"
)

// main function.
func main() {
	command := NewRootCommand()
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

func NewRootCommand() *cobra.Command {
	tanglecli := cli.New()

	cmd := &cobra.Command{
		Use:     "tangle-cli",
		Short:   "CLI client for the Tangle server",
		Long:    "Leverage tangle from inside your CLI pipelines.",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			tanglecli.Out = cmd.OutOrStdout()
			tanglecli.Err = cmd.ErrOrStderr()

			return initializeConfig(cmd)
		},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprint(tanglecli.Out, cmd.UsageString())
		},
	}

	cmd.PersistentFlags().StringVar(&tanglecli.Config.ServerAddr, "server-address", "", "ArgoCD server address")
	cmd.PersistentFlags().BoolVar(&tanglecli.Config.Insecure, "insecure", false, "Don't validate SSL certificate on client request")
	cmd.PersistentFlags().StringSliceVar(&tanglecli.Config.LabelsAsStrings, "label", []string{}, "Labels to filter projects on in format 'key=value'.  Can be used multiple times.")

	return cmd
}

func initializeConfig(cmd *cobra.Command) error {
	v := viper.New()

	v.SetConfigName(defaultConfigFilename)
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()
	bindFlags(cmd, v)

	return nil
}

func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if strings.Contains(f.Name, "-") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			if err := v.BindEnv(f.Name, fmt.Sprintf("%s_%s", envPrefix, envVarSuffix)); err != nil {
				os.Exit(1)
			}
		}

		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			if err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val)); err != nil {
				os.Exit(1)
			}
		}
	})
}
