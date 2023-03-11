package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:          "gphotobackup",
	SilenceUsage: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.houselbot.yaml)")
	//rootCmd.PersistentFlags().String("articles", "", "Which article list to choose from")
	//_ = viper.BindPFlag("articles", rootCmd.PersistentFlags().Lookup("articles"))
	//
	//rootCmd.PersistentFlags().StringSlice("email.to", []string{}, "Also email these addresses")
	//_ = viper.BindPFlag("email.to", rootCmd.PersistentFlags().Lookup("email.to"))
	//
	//viper.SetDefault("email.host", "smtp.gmail.com")
	//viper.SetDefault("email.port", 587)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".houselbot" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".gphotobackup")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
