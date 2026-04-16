package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aarondpn/redmine-cli/internal/cmd"
	"github.com/aarondpn/redmine-cli/internal/cmd/update"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
)

var version = "dev"

const (
	updateCheckHintDelay = 200 * time.Millisecond
	updateCheckMaxWait   = 2 * time.Second
)

func main() {
	// Start background update check.
	var updateDone chan *update.CheckResult
	var cancelUpdateCheck context.CancelFunc
	if update.ShouldCheck(version, os.Args[1:]) {
		updateDone = make(chan *update.CheckResult, 1)
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckMaxWait)
		cancelUpdateCheck = cancel
		go func() {
			updateDone <- update.CheckForUpdateCached(ctx, version)
		}()
	}

	rootCmd, factory := cmd.NewRootCmdWithFactory(version)
	err := rootCmd.Execute()

	if err != nil {
		if cancelUpdateCheck != nil {
			cancelUpdateCheck()
		}
		var silent *cmdutil.SilentError
		if errors.As(err, &silent) {
			os.Exit(silent.Code)
		}
		selectedFormat := ""
		if factory != nil {
			selectedFormat = factory.OutputFormat
		}
		if selectedFormat == "" {
			// Resolve the leaf command cobra was asked to run so we pick up
			// --output from a local flag (shadowing the persistent one) or
			// from commands that failed before PersistentPreRunE (e.g.
			// cobra's Args validation).
			if targetCmd, _, findErr := rootCmd.Find(os.Args[1:]); findErr == nil && targetCmd != nil {
				if of := targetCmd.Flags().Lookup("output"); of != nil {
					selectedFormat = of.Value.String()
				}
			}
		}
		if selectedFormat == "" {
			if of := rootCmd.PersistentFlags().Lookup("output"); of != nil {
				selectedFormat = of.Value.String()
			}
		}
		if selectedFormat == output.FormatJSON {
			_ = output.RenderErrorJSON(os.Stdout, cmdutil.BuildErrorEnvelope(err))
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", cmdutil.FormatError(err))
		}
		os.Exit(1)
	}

	waitForStartupUpdate(os.Stderr, version, updateDone, cancelUpdateCheck, updateCheckHintDelay, updateCheckMaxWait)
}

func waitForStartupUpdate(w io.Writer, currentVersion string, updateDone <-chan *update.CheckResult, cancel context.CancelFunc, hintDelay, maxWait time.Duration) {
	if updateDone == nil {
		return
	}
	if cancel != nil {
		defer cancel()
	}

	select {
	case result := <-updateDone:
		if result != nil {
			update.PrintNotice(w, currentVersion, result)
		}
		return
	case <-time.After(hintDelay):
		fmt.Fprintln(w, "Checking for updates...")
	case <-time.After(maxWait):
		if cancel != nil {
			cancel()
		}
		return
	}

	select {
	case result := <-updateDone:
		if result != nil {
			update.PrintNotice(w, currentVersion, result)
		}
	case <-time.After(maxWait - hintDelay):
		if cancel != nil {
			cancel()
		}
	}
}
