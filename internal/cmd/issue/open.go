package issue

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// openBrowser is a package-level variable so tests can stub it.
var openBrowser = func(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Run()
}

// NewCmdOpen creates the issues open command.
func NewCmdOpen(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "open <id>",
		Short: "Open an issue in the browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			cfg, err := f.Config()
			if err != nil {
				return err
			}

			url := fmt.Sprintf("%s/issues/%d", strings.TrimRight(cfg.Server, "/"), id)

			if err := openBrowser(url); err != nil {
				return fmt.Errorf("failed to open browser: %w", err)
			}

			printer := f.Printer(format)
			printer.Action(output.ActionOpened, "issue", id, fmt.Sprintf("Opened issue #%d in browser (%s)", id, url))
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
