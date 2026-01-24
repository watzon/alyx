package cli

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "alyx",
	Short: "A portable, polyglot Backend-as-a-Service",
	Long: `Alyx is a Backend-as-a-Service (BaaS) platform that provides:

  - Single Go binary deployment (like PocketBase)
  - SQLite database with optional Turso for distributed deployments
  - YAML-defined schema with automatic migrations and type-safe client generation
  - Real-time subscriptions via WebSocket with efficient change detection
  - Container-based serverless functions supporting multiple languages
  - CEL-based access control for fine-grained security rules

Start the development server:
  alyx dev

Initialize a new project:
  alyx init my-app`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./alyx.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in current directory
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("alyx")
	}

	// Read in environment variables that match
	viper.SetEnvPrefix("ALYX")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			log.Debug().Str("file", viper.ConfigFileUsed()).Msg("Using config file")
		}
	}
}

// setupLogging configures zerolog based on verbosity and environment.
func setupLogging() {
	// Pretty console output for development
	output := zerolog.ConsoleWriter{Out: os.Stderr}

	if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

// AddCommand adds a command to the root command.
func AddCommand(cmd *cobra.Command) {
	rootCmd.AddCommand(cmd)
}

// Version returns the version string.
func Version() string {
	return fmt.Sprintf("alyx version %s", "0.1.0-dev")
}
