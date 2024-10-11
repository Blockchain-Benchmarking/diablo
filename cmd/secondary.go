/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"diablo/core"
	"diablo/core/secondary"
	"github.com/spf13/cobra"
)

var secondaryConnectPort int
var tags []string
var prim string

// secondaryCmd represents the secondary command
var secondaryCmd = &cobra.Command{
	Use:   "secondary",
	Short: "Launch a Diablo secondary node",
	Long: `Launch a Diablo secondary node to run the benchmark following the
    directives of the Diablo primary node at the specified <primary> address.`,
	Run: func(cmd *cobra.Command, args []string) {
		core.SetVerbosity(verbosity)
		err := validatePort(secondaryConnectPort)
		cobra.CheckErr(err)

		secondary, err := secondary.NewSecondary(prim, secondaryConnectPort, tags)
		cobra.CheckErr(err)

		err = secondary.Run()
		cobra.CheckErr(err)
	},
}

func init() {
	rootCmd.AddCommand(secondaryCmd)

	secondaryCmd.Flags().IntVarP(&secondaryConnectPort, "port", "p", PORT_DEFAULT, "Port to listen for Diablo secondary nodes on.")
	secondaryCmd.Flags().StringSliceVarP(&tags, "tags", "t", nil, "Attach given tags to the node.")
	secondaryCmd.Flags().StringVar(&prim, "primary", "127.0.0.1", "Primary address.")
}
