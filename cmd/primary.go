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

const (
	defaultPort     int = 5000
	defaultDuration     = 2 * time.Minute

	defaultBenchmark  = "simple"
	defaultUser       = "stubbornPaymentUser"
	defaultBlockchain = "ethereum"
)

var compress bool
var outputFile, accountsFile string
var port, secondaries int

var benchmark string
var duration time.Duration

var tps int
var userType string
var blockchain string
var endpoints []string
var configFile string

var primaryCmd = &cobra.Command{
	Use:   "primary",
	Short: "Launch a Diablo primary node",
	Long: `Launch a Diablo primary node to run the benchmark specified by the <benchmark> flag
     using a number of Diablo secondary nodes specified by the <secondaries> flag and a sending rate specified by the <tps> flag. The list of blockchain
	 endpoints is specified by the <endpoints> flag.`,
	Run: func(cmd *cobra.Command, args []string) {
		var output io.WriteCloser

		core.SetVerbosity(verbosity)

		p, err := nodes.NewPrimary(port, secondaries, benchmark, configFile, accountsFile, duration, tps, userType, blockchain, endpoints)
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

	primaryCmd.Flags().IntVarP(&secondaries, "secondaries", "s", 0, "Set the number of secondaries.")
	err := primaryCmd.MarkFlagRequired("secondaries")
	cobra.CheckErr(err)

	primaryCmd.Flags().StringVarP(&benchmark, "benchmark", "b", defaultBenchmark, "Set the benchmark type.")
	primaryCmd.Flags().DurationVarP(&duration, "duration", "d", defaultDuration, "Set the experiment duration.")

	primaryCmd.Flags().StringVar(&configFile, "config", "", "Set user / blockchain config file.")

	primaryCmd.Flags().IntVarP(&tps, "tps", "t", 0, "Set sending rate in transactions per second.")
	err = primaryCmd.MarkFlagRequired("tps")
	cobra.CheckErr(err)

	primaryCmd.Flags().StringVarP(&userType, "user", "u", defaultUser, "Set user type.")
	primaryCmd.Flags().StringVar(&blockchain, "blockchain", defaultBlockchain, "Set blockchain.")

	primaryCmd.Flags().StringArrayVarP(&endpoints, "endpoints", "e", []string{}, "Set blockchain endpoints.")
	err = primaryCmd.MarkFlagRequired("endpoints")

	primaryCmd.PersistentFlags().StringVar(&accountsFile, "accounts", "", "Set accounts file path.")
	err = primaryCmd.MarkFlagRequired("accounts")
	cobra.CheckErr(err)
}
