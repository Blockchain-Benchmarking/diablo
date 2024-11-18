/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultPort     int           = 5000
	defaultDuration time.Duration = 2 * time.Minute

	defaultBenchmark = "simple"
	defaultTps       = 2000
)

var (
	defaultEndpoints = []string{"ws://127.0.0.1:9000", "ws://127.0.0.1:9001", "ws://127.0.0.1:9002"}
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
