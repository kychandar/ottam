package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	env     string
	rootCmd = &cobra.Command{
		Use:   "processor",
		Short: "Async message processor that helps in fanning out the messages",
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: config/config.yaml)")
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	if cfgFile != "" {
		// Use the specified config file
		viper.SetConfigFile(cfgFile)
	} else {
		// Default location: $HOME/.myapp.yaml
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".myapp")
	}

	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
