package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/aarondpn/redmine-cli/internal/cmd"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

var version = "dev"

func main() {
	rootCmd := cmd.NewRootCmd(version)
	if err := rootCmd.Execute(); err != nil {
		var silent *cmdutil.SilentError
		if errors.As(err, &silent) {
			os.Exit(silent.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", cmdutil.FormatError(err))
		os.Exit(1)
	}
}
