package main

import (
	"os"

	"github.com/ethpandaops/eth-snapshotter/internal/config"
	"github.com/ethpandaops/eth-snapshotter/internal/server"
	"github.com/ethpandaops/eth-snapshotter/internal/snapshotter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "snapshotter",
	Short: "snapshotter",
	Run: func(cmd *cobra.Command, args []string) {
		cfgPath, _ := cmd.Flags().GetString("config")
		cfg, err := config.ReadFromFile(cfgPath)
		if err != nil {
			log.WithError(err).Fatal("failed reading config")
		}
		ss, err := snapshotter.Init(cfg)
		if err != nil {
			log.WithError(err).Fatal("failed to start")
		}

		// Initialize HTTP server
		srv := server.New(cfg, ss.GetDB(), ss.GetStatus)
		go func() {
			if err := srv.Start(); err != nil {
				log.WithError(err).Fatal("failed to start HTTP server")
			}
		}()

		ss.StartPeriodicPolling()
	},
}

func init() {
	rootCmd.PersistentFlags().String("config", "config.yaml", "config file")
	//log.SetLevel(log.DebugLevel) // Todo. parse from config file
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
