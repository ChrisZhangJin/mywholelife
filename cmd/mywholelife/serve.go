package main

import (
	"github.com/spf13/cobra"

	"mywholelife/server"
	"mywholelife/store"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the mywholelife HTTP API",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := server.FromEnv()
		blobs := store.NewBlobStore(cfg.DataRoot)
		st, err := store.Open(cfg.DBPath, blobs)
		if err != nil {
			return err
		}
		return server.NewRouter(st, blobs).Run(cfg.Addr)
	},
}
