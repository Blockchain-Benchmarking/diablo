package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var accountsFile string
var verbosity int

var rootCmd = &cobra.Command{
	Use:     "diablo",
	Short:   "Launch a Diablo node",
	Version: "v4",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&accountsFile, "accounts", "", "Set accounts file path.")
	err := rootCmd.MarkFlagRequired("accounts")
	cobra.CheckErr(err)

	rootCmd.PersistentFlags().IntVar(&verbosity, "verbose", 4, "Set verbosity to <lvl> (fatal=1,"+
		"error=2, warning=3, info=4, debug=5, trace=6).")

	rootCmd.Flags().BoolP("version", "v", false, "Display latest version.")
}
