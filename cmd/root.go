/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"diablo/core"
	"os"

	"github.com/spf13/cobra"
)

const (
	VERBOSITY_SILENT  int = 0
	VERBOSITY_FATAL   int = 1
	VERBOSITY_ERROR   int = 2
	VERBOSITY_WARNING int = 3
	VERBOSITY_INFO    int = 4
	VERBOSITY_DEBUG   int = 5
	VERBOSITY_TRACE   int = 6

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

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
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

func setVerbosity(verbosity int) {
	var level core.LogLevel
	var logger core.Logger

	if verbosity == VERBOSITY_SILENT {
		level = core.LOG_SILENT
	} else if verbosity == VERBOSITY_FATAL {
		level = core.LOG_FATAL
	} else if verbosity == VERBOSITY_ERROR {
		level = core.LOG_ERROR
	} else if verbosity == VERBOSITY_WARNING {
		level = core.LOG_WARN
	} else if verbosity == VERBOSITY_INFO {
		level = core.LOG_INFO
	} else if verbosity == VERBOSITY_DEBUG {
		level = core.LOG_DEBUG
	} else {
		level = core.LOG_TRACE
	}

	logger = core.NewPrintLogger(os.Stderr, "", level)

	core.SetLogger(logger)
}
