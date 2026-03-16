package main

import (
	"fmt"
	"os"

	"github.com/aarondpn/redmine-cli/internal/cmd"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

var version = "dev"

func main() {
	rootCmd := cmd.NewRootCmd(version)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", cmdutil.FormatError(err))
		os.Exit(1)
	}
}
