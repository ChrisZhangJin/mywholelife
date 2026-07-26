package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

var dreamCmd = &cobra.Command{
	Use:   "dream",
	Short: "Run the long-term memory maintenance job (reserved)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("dream: not implemented until phase 4")
	},
}

func main() {
	root := &cobra.Command{Use: "mywholelife"}
	root.AddCommand(serveCmd, dreamCmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
