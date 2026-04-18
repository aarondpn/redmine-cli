package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type outputContract string

const (
	outputContractStructured     outputContract = "structured"
	outputContractRawPassthrough outputContract = "raw_passthrough"
	outputContractInteractive    outputContract = "interactive_only"
)

type commandContract struct {
	Mode   outputContract
	Reason string
}

// commandOutputContracts is an explicit registry for every runnable,
// user-facing command. Adding a new command requires classifying how it
// handles output, which turns "forgotten command" regressions into a test
// failure instead of a review surprise.
var commandOutputContracts = map[string]commandContract{
	"redmine api":                {Mode: outputContractStructured},
	"redmine auth list":          {Mode: outputContractStructured},
	"redmine auth login":         {Mode: outputContractInteractive, Reason: "runs an interactive Huh form to collect credentials"},
	"redmine auth logout":        {Mode: outputContractInteractive, Reason: "may prompt for interactive confirmation before deleting a profile"},
	"redmine auth status":        {Mode: outputContractStructured},
	"redmine auth switch":        {Mode: outputContractInteractive, Reason: "may prompt with an interactive profile selector"},
	"redmine categories list":    {Mode: outputContractStructured},
	"redmine completion":         {Mode: outputContractRawPassthrough, Reason: "streams generated shell completion scripts"},
	"redmine config":             {Mode: outputContractStructured},
	"redmine groups add-user":    {Mode: outputContractStructured},
	"redmine groups create":      {Mode: outputContractStructured},
	"redmine groups delete":      {Mode: outputContractStructured},
	"redmine groups get":         {Mode: outputContractStructured},
	"redmine groups list":        {Mode: outputContractStructured},
	"redmine groups remove-user": {Mode: outputContractStructured},
	"redmine groups update":      {Mode: outputContractStructured},
	"redmine install-skill":      {Mode: outputContractStructured},
	"redmine issues assign":      {Mode: outputContractStructured},
	"redmine issues browse":      {Mode: outputContractInteractive, Reason: "launches the TUI browser"},
	"redmine issues close":       {Mode: outputContractStructured},
	"redmine issues comment":     {Mode: outputContractStructured},
	"redmine issues create":      {Mode: outputContractStructured},
	"redmine issues delete":      {Mode: outputContractStructured},
	"redmine issues get":         {Mode: outputContractStructured},
	"redmine issues list":        {Mode: outputContractStructured},
	"redmine issues open":        {Mode: outputContractStructured},
	"redmine issues reopen":      {Mode: outputContractStructured},
	"redmine issues update":      {Mode: outputContractStructured},
	"redmine mcp serve":          {Mode: outputContractRawPassthrough, Reason: "runs the MCP JSON-RPC protocol over stdio"},
	"redmine memberships create": {Mode: outputContractStructured},
	"redmine memberships delete": {Mode: outputContractStructured},
	"redmine memberships get":    {Mode: outputContractStructured},
	"redmine memberships list":   {Mode: outputContractStructured},
	"redmine memberships update": {Mode: outputContractStructured},
	"redmine projects create":    {Mode: outputContractStructured},
	"redmine projects delete":    {Mode: outputContractStructured},
	"redmine projects get":       {Mode: outputContractStructured},
	"redmine projects list":      {Mode: outputContractStructured},
	"redmine projects members":   {Mode: outputContractStructured},
	"redmine projects update":    {Mode: outputContractStructured},
	"redmine search":             {Mode: outputContractStructured},
	"redmine search browse":      {Mode: outputContractInteractive, Reason: "launches the TUI search browser"},
	"redmine search issues":      {Mode: outputContractStructured},
	"redmine search messages":    {Mode: outputContractStructured},
	"redmine search news":        {Mode: outputContractStructured},
	"redmine search projects":    {Mode: outputContractStructured},
	"redmine search wiki":        {Mode: outputContractStructured},
	"redmine statuses list":      {Mode: outputContractStructured},
	"redmine time delete":        {Mode: outputContractStructured},
	"redmine time get":           {Mode: outputContractStructured},
	"redmine time list":          {Mode: outputContractStructured},
	"redmine time log":           {Mode: outputContractStructured},
	"redmine time summary":       {Mode: outputContractStructured},
	"redmine time update":        {Mode: outputContractStructured},
	"redmine trackers list":      {Mode: outputContractStructured},
	"redmine update":             {Mode: outputContractStructured},
	"redmine users create":       {Mode: outputContractStructured},
	"redmine users delete":       {Mode: outputContractStructured},
	"redmine users get":          {Mode: outputContractStructured},
	"redmine users list":         {Mode: outputContractStructured},
	"redmine users me":           {Mode: outputContractStructured},
	"redmine users update":       {Mode: outputContractStructured},
	"redmine versions get":       {Mode: outputContractStructured},
	"redmine versions list":      {Mode: outputContractStructured},
	"redmine wiki create":        {Mode: outputContractStructured},
	"redmine wiki delete":        {Mode: outputContractStructured},
	"redmine wiki get":           {Mode: outputContractStructured},
	"redmine wiki list":          {Mode: outputContractStructured},
	"redmine wiki update":        {Mode: outputContractStructured},
}

func TestRunnableCommandsHaveDeclaredOutputContracts(t *testing.T) {
	root := NewRootCmd("test")

	var actual []string
	walk(root, func(c *cobra.Command) {
		if c.Runnable() && !c.Hidden {
			actual = append(actual, c.CommandPath())
		}
	})
	slices.Sort(actual)

	var expected []string
	for commandPath, contract := range commandOutputContracts {
		expected = append(expected, commandPath)
		if contract.Mode == outputContractRawPassthrough || contract.Mode == outputContractInteractive {
			if strings.TrimSpace(contract.Reason) == "" {
				t.Fatalf("%s must include a reason for %s classification", commandPath, contract.Mode)
			}
		}
	}
	slices.Sort(expected)

	if !slices.Equal(actual, expected) {
		t.Fatalf("runnable command output contract registry drifted\nactual:   %v\nexpected: %v", actual, expected)
	}
}

func TestDirectStdoutWritesAreAllowlisted(t *testing.T) {
	allowed := map[string]string{
		"completion/completion.go": "shell completion generator streams directly to stdout",
		"issue/get.go":             "human journal rendering still prints directly after detail output",
		"update/check.go":          "background update notice writes directly to an arbitrary writer",
	}
	patterns := []string{
		"fmt.Print(",
		"fmt.Printf(",
		"fmt.Println(",
		"fmt.Fprintf(f.IOStreams.Out",
		"fmt.Fprint(f.IOStreams.Out",
		"fmt.Fprintln(f.IOStreams.Out",
		"os.Stdout",
	}

	rootDir := "."
	var offenders []string

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(data)
		for _, pattern := range patterns {
			if strings.Contains(src, pattern) {
				if _, ok := allowed[relPath]; !ok {
					offenders = append(offenders, relPath+" contains "+pattern)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk command sources: %v", err)
	}

	if len(offenders) > 0 {
		slices.Sort(offenders)
		t.Fatalf("direct stdout/stderr writes found outside the allowlist:\n%s", strings.Join(offenders, "\n"))
	}
}
