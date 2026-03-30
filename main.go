package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aarondpn/redmine-cli/internal/cmd"
	"github.com/aarondpn/redmine-cli/internal/cmd/update"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

var version = "dev"

func main() {
	// Start background update check.
	var updateDone chan *update.CheckResult
	if update.ShouldCheck(version, os.Args[1:]) {
		updateDone = make(chan *update.CheckResult, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			updateDone <- update.CheckForUpdate(ctx, version)
		}()
	}

	rootCmd := cmd.NewRootCmd(version)
	err := rootCmd.Execute()

	// Print update notice after command output.
	if updateDone != nil {
		if result := <-updateDone; result != nil {
			update.PrintNotice(os.Stderr, version, result)
		}
	}

	if err != nil {
		var silent *cmdutil.SilentError
		if errors.As(err, &silent) {
			os.Exit(silent.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", cmdutil.FormatError(err))
		os.Exit(1)
	}
}
