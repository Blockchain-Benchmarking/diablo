/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"compress/gzip"
	"diablo/core"
	"github.com/spf13/cobra"
	"io"
	"os"
	"strings"
	"time"
)

var compress bool
var outputFile, setupFile, benchmark string
var port, secondaries int
var duration time.Duration

// primaryCmd represents the primary command
var primaryCmd = &cobra.Command{
	Use:   "primary",
	Short: "Launch a Diablo primary node",
	Long: `Launch a Diablo primary node to run the benchmark specified by the given
    <benchmark> configuration file on the setup specified by the <setup> file
    using a number <nsecondary> of Diablo secondary node`,
	Run: func(cmd *cobra.Command, args []string) {
		var output io.WriteCloser

		core.SetVerbosity(verbosity)

		p, err := core.NewPrimary(port, secondaries, benchmark, setupFile, accountsFile, duration)
		cobra.CheckErr(err)

		if outputFile != "" {
			if compress && !strings.HasSuffix(outputFile, ".gz") {
				outputFile += ".gz"
			}

			output, err = os.Create(outputFile)
			cobra.CheckErr(err)

			defer output.Close()
		} else {
			output = os.Stdout
		}

		if compress {
			output = gzip.NewWriter(output)
			defer output.Close()
		}

		result, err := p.Run()
		cobra.CheckErr(err)

		err = result.PrintResult(output)
		cobra.CheckErr(err)
	},
}

func init() {
	rootCmd.AddCommand(primaryCmd)

	primaryCmd.Flags().BoolVar(&compress, "compress", false, "Compress output with gzip. "+
		"Add a '.gz' suffix to the output path is not already present.")
	primaryCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write results in the file instead of "+
		"printing on standard output.")
	primaryCmd.Flags().StringVar(&setupFile, "setup", "", "setup file")
	primaryCmd.Flags().IntVarP(&port, "port", "p", PORT_DEFAULT, "Port to listen for Diablo secondary nodes on.")

	primaryCmd.Flags().IntVar(&secondaries, "secondaries", 0, "number of secondaries")

	primaryCmd.Flags().StringVar(&benchmark, "benchmark", "simple", "benchmark to use")
	primaryCmd.Flags().DurationVarP(&duration, "duration", "d", time.Minute*2, "experiment duration")

	//TODO mark mandatory flags
}
