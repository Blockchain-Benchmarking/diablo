/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"compress/gzip"
	"diablo/cmd/nodes"
	"diablo/core"
	"diablo/core/logging"
	"encoding/json"
	"github.com/spf13/cobra"
	"io"
	"os"
	"strings"
	"time"
)

var compress bool
var outputFile, benchmark string
var port, secondaries int
var duration time.Duration
var configFile string

var tps int
var endpoints []string

//var simpleFlags SimpleFlags

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

		p, err := nodes.NewPrimary(port, secondaries, benchmark, configFile, accountsFile, duration, tps, endpoints)
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

		logging.Infof("writing results in output")

		for _, res := range result {
			buf, err := json.Marshal(res)
			cobra.CheckErr(err)
			_, err = output.Write(buf)
			cobra.CheckErr(err)
		}

		logging.Infof("primary done")

		err = output.Close()
		cobra.CheckErr(err)
	},
}

func init() {
	rootCmd.AddCommand(primaryCmd)

	primaryCmd.Flags().BoolVar(&compress, "compress", false, "Compress output with gzip. "+
		"Add a '.gz' suffix to the output path is not already present.")
	primaryCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write results in the file instead of "+
		"printing on standard output.")
	primaryCmd.Flags().IntVarP(&port, "port", "p", defaultPort, "Port to listen for Diablo secondary nodes on.")

	primaryCmd.Flags().IntVar(&secondaries, "secondaries", 0, "number of secondaries")

	primaryCmd.Flags().StringVar(&benchmark, "benchmark", defaultBenchmark, "benchmark to use")
	primaryCmd.Flags().DurationVarP(&duration, "duration", "d", defaultDuration, "experiment duration")

	primaryCmd.Flags().StringVar(&configFile, "config", "", "config file")

	primaryCmd.Flags().IntVar(&tps, "tps", defaultTps, "transactions per second")
	primaryCmd.Flags().StringArrayVarP(&endpoints, "endpoints", "e", defaultEndpoints, "endpoints")

	//TODO mark mandatory flags
}
