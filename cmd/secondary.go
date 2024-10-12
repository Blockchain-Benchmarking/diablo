/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"diablo/core"
	"fmt"
	"github.com/spf13/cobra"
)

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

		secondary, err := core.NewSecondary(prim, tags)
		cobra.CheckErr(err)

		err = secondary.Run()
		cobra.CheckErr(err)
	},
}

func init() {
	rootCmd.AddCommand(secondaryCmd)

	secondaryCmd.Flags().StringSliceVarP(&tags, "tags", "t", nil, "Attach given tags to the node.")
	secondaryCmd.Flags().StringVar(&prim, "primary", fmt.Sprintf("127.0.0.1:%d", PORT_DEFAULT), "Primary address.")
}
