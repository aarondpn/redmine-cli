package main

import (
	"os"

	"github.com/aarondpn/redmine-cli/internal/cmd"
)

var version = "dev"

func main() {
	rootCmd := cmd.NewRootCmd(version)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
