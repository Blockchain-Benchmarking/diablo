/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	PORT_DEFAULT int = 5000

	MAX_DELAY_DEFAULT float64 = 1.0
	MAX_SKEW_DEFAULT  float64 = 0.2
)

var accountsFile string
var verbosity int

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "diablo",
	Short:   "Diablo benchmark",
	Version: "TODO",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&accountsFile, "accounts", "", "Set accounts file path.")
	rootCmd.PersistentFlags().IntVar(&verbosity, "verbose", 4, "Set verbosity to <lvl> (fatal=1,"+
		"error=2, warning=3, info=4, debug=5, trace=6).")

	rootCmd.Flags().BoolP("version", "v", false, "Display latest version.")
}
