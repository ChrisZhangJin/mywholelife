package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{Use: "mywholelife"}
	root.AddCommand(serveCmd, dreamCmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
