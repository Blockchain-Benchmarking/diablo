/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"diablo/core"
	"fmt"

	"github.com/spf13/cobra"
)

var secondaryConnectPort int
var tags []string
var primary string

// secondaryCmd represents the secondary command
var secondaryCmd = &cobra.Command{
	Use:   "secondary",
	Short: "Launch a Diablo secondary node",
	Long: `Launch a Diablo secondary node to run the benchmark following the
    directives of the Diablo primary node at the specified <primary> address.`,
	Run: func(cmd *cobra.Command, args []string) {
		setVerbosity(verbosity)
		err := validatePort(secondaryConnectPort)
		cobra.CheckErr(err)

		secondary, err := core.NewSecondary(primary, port, tags)
		cobra.CheckErr(err)

		err = secondary.Run()
		cobra.CheckErr(err)

		fmt.Println("secondary called")
	},
}

func init() {
	rootCmd.AddCommand(secondaryCmd)

	primaryCmd.Flags().StringSliceVarP(&tags, "tags", "t", nil, "Attach given tags to the node.")
	primaryCmd.Flags().IntVarP(&secondaryConnectPort, "port", "p", PORT_DEFAULT, "Port to connect to the Diablo primary node on.")
	primaryCmd.Flags().StringVar(&primary, "primary", "127.0.0.1", "Primary address.")
}
