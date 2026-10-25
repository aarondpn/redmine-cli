package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/mcpserver"
)

func newCmdServe(f *cmdutil.Factory) *cobra.Command {
	var (
		enableWrites  bool
		name          string
		httpAddr      string
		authToken     string
		authTokenFile string
		noAuth        bool
		enableGroups  []string
		disableGroups []string
		enableTools   []string
		disableTools  []string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Redmine MCP server on stdio or HTTP",
		Long: "Start a Model Context Protocol server on stdio or streamable HTTP that exposes the " +
			"Redmine API as a set of MCP tools and resources. The active " +
			"--profile (or REDMINE_* environment variables / --server / --api-key " +
			"flags) selects the Redmine instance.\n\n" +
			"By default only read tools are registered. Pass --enable-writes to " +
			"also register create/update/delete tools. Use --http to listen on " +
			"an HTTP address such as :8080 instead of stdio. The HTTP transport " +
			"defaults to a loopback bind (127.0.0.1); pass an explicit host to " +
			"listen elsewhere. Every request must then carry a bearer token: set " +
			"--auth-token-file (or REDMINE_MCP_AUTH_TOKEN), otherwise a random token " +
			"is generated at startup and printed to stderr. Pass --no-auth only " +
			"when something in front of the server (e.g. a reverse proxy) " +
			"already authenticates clients. A loopback bind without a token " +
			"stays open to every local user and process; the server warns about " +
			"it, and in strict mode generates a token instead.\n\n" +
			"To narrow the set of tools exposed to MCP clients, use " +
			"--enable-groups / --disable-groups (allow- and deny-list of tool " +
			"groups) and --enable-tools / --disable-tools (per-tool overrides). " +
			"Run `redmine mcp tools` to see the available groups and tools.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			version := cmd.Root().Version
			if version == "" {
				version = "dev"
			}

			cfg, _ := f.Config()
			spec, err := buildFilterSpec(cfg, cmd, enableGroups, disableGroups, enableTools, disableTools)
			if err != nil {
				return err
			}

			opts := mcpserver.Options{
				EnableWrites: resolveEnableWrites(cmd, enableWrites, cfg),
				Name:         name,
				Version:      version,
			}
			opts = mcpserver.ResolveOptions(opts, spec)

			// main already cancels cmd.Context() on the first signal; this
			// keeps serve shutting down cleanly when it runs without main
			// (tests, embedding).
			ctx, stop := signal.NotifyContext(cmdutil.Context(cmd), os.Interrupt, syscall.SIGTERM)
			defer stop()

			httpAddr = resolveHTTPAddr(cmd, httpAddr, cfg)
			if httpAddr != "" {
				normalized, isLoopback := mcpserver.NormalizeBindAddr(httpAddr)
				token, generated, err := resolveHTTPAuth(f, cmd, cfg, isLoopback, httpAuthFlags{
					Token:     authToken,
					TokenFile: authTokenFile,
					NoAuth:    noAuth,
				})
				if err != nil {
					return err
				}

				server := mcpserver.BuildHTTPServer(client, opts, mcpserver.HTTPOptions{
					Addr:      normalized,
					AuthToken: token,
				})
				ln, err := net.Listen("tcp", normalized)
				if err != nil {
					return err
				}
				go func() {
					<-ctx.Done()
					_ = server.Close()
				}()
				// Only announce a generated token once the bind succeeded, so a
				// bad address doesn't print a token that is never used.
				if generated {
					printGeneratedToken(cmd.ErrOrStderr(), token, isLoopback)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "MCP server listening on http://%s\n", listenAddr(normalized, ln.Addr()))
				err = server.Serve(ln)
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			}

			srv := mcpserver.BuildServer(client, opts)
			return srv.Run(ctx, &sdk.StdioTransport{})
		},
	}

	cmd.Flags().BoolVar(&enableWrites, "enable-writes", false, "Register tools that create, update, or delete Redmine data")
	cmd.Flags().StringVar(&httpAddr, "http", "", "Serve MCP over streamable HTTP on the given address instead of stdio (e.g. :8080 binds 127.0.0.1; pass 0.0.0.0:8080 to expose externally). Also settable via REDMINE_MCP_HTTP.")
	cmd.Flags().StringVar(&authToken, "auth-token", "", "Require this bearer token in the Authorization header for every HTTP request. When --http binds outside of loopback and no token is set, one is generated at startup. Visible in the process list; prefer --auth-token-file or REDMINE_MCP_AUTH_TOKEN.")
	cmd.Flags().StringVar(&authTokenFile, "auth-token-file", "", "Read the --auth-token value from a file, or from standard input with \"-\", so it never appears in the process list.")
	cmd.MarkFlagsMutuallyExclusive("auth-token", "auth-token-file")
	cmd.Flags().BoolVar(&noAuth, "no-auth", false, "Serve a non-loopback --http bind without a bearer token instead of generating one. Only use behind an authenticating proxy; rejected in strict mode.")
	cmd.Flags().StringVar(&name, "name", "redmine-cli", "Server name advertised to MCP clients")
	cmd.Flags().StringSliceVar(&enableGroups, "enable-groups", nil, "Comma-separated tool groups to expose (default: all). See redmine mcp tools.")
	cmd.Flags().StringSliceVar(&disableGroups, "disable-groups", nil, "Comma-separated tool groups to hide. Applied after --enable-groups.")
	cmd.Flags().StringSliceVar(&enableTools, "enable-tools", nil, "Allow-list of tool names. Tools outside the list are hidden.")
	cmd.Flags().StringSliceVar(&disableTools, "disable-tools", nil, "Deny-list of tool names. Applied after --enable-tools.")

	cmdutil.MarkNoJSONOutput(cmd)
	return cmd
}

// buildFilterSpec merges config-file values with the CLI flag values. CLI
// flags take precedence: when a flag is explicitly set, the corresponding
// config value is ignored entirely (rather than appended).
func buildFilterSpec(cfg *config.Config, cmd *cobra.Command, enableGroupsFlag, disableGroupsFlag, enableToolsFlag, disableToolsFlag []string) (mcpserver.FilterSpec, error) {
	pick := func(flagName string, fromFlag, fromCfg []string) []string {
		if cmd.Flags().Changed(flagName) {
			return fromFlag
		}
		return fromCfg
	}

	var cfgEG, cfgDG, cfgET, cfgDT []string
	if cfg != nil {
		cfgEG = cfg.MCP.EnableGroups
		cfgDG = cfg.MCP.DisableGroups
		cfgET = cfg.MCP.EnableTools
		cfgDT = cfg.MCP.DisableTools
	}

	eg := pick("enable-groups", enableGroupsFlag, cfgEG)
	dg := pick("disable-groups", disableGroupsFlag, cfgDG)
	et := pick("enable-tools", enableToolsFlag, cfgET)
	dt := pick("disable-tools", disableToolsFlag, cfgDT)

	parsedEnable, err := mcpserver.ParseGroups(eg)
	if err != nil {
		return mcpserver.FilterSpec{}, err
	}
	parsedDisable, err := mcpserver.ParseGroups(dg)
	if err != nil {
		return mcpserver.FilterSpec{}, err
	}
	parsedEnableTools, err := mcpserver.ParseTools(et)
	if err != nil {
		return mcpserver.FilterSpec{}, err
	}
	parsedDisableTools, err := mcpserver.ParseTools(dt)
	if err != nil {
		return mcpserver.FilterSpec{}, err
	}

	return mcpserver.FilterSpec{
		EnableGroups:  parsedEnable,
		DisableGroups: parsedDisable,
		EnableTools:   parsedEnableTools,
		DisableTools:  parsedDisableTools,
	}, nil
}

// resolveEnableWrites returns the effective --enable-writes setting. Flag
// takes precedence; otherwise the config-file MCP block wins; otherwise the
// flag default.
func resolveEnableWrites(cmd *cobra.Command, flagVal bool, cfg *config.Config) bool {
	if cmd.Flags().Changed("enable-writes") {
		return flagVal
	}
	if cfg != nil && cfg.MCP.EnableWrites != nil {
		return *cfg.MCP.EnableWrites
	}
	return flagVal
}

// resolveAuthToken returns the effective HTTP bearer token. Precedence: CLI
// flag > config block. The env-var override (REDMINE_MCP_AUTH_TOKEN) is
// applied to the config block in applyEnvOverrides.
func resolveAuthToken(cmd *cobra.Command, flagVal string, cfg *config.Config) string {
	if cmd.Flags().Changed("auth-token") {
		return flagVal
	}
	if cfg != nil && cfg.MCP.AuthToken != "" {
		return cfg.MCP.AuthToken
	}
	return flagVal
}

// resolveHTTPAddr returns the effective --http address. Precedence: CLI flag >
// config block. The env-var override (REDMINE_MCP_HTTP) is applied to the
// config block in applyEnvOverrides. An empty result means stdio.
func resolveHTTPAddr(cmd *cobra.Command, flagVal string, cfg *config.Config) string {
	if cmd.Flags().Changed("http") {
		return flagVal
	}
	if cfg != nil && cfg.MCP.HTTP != "" {
		return cfg.MCP.HTTP
	}
	return flagVal
}

// resolveNoAuth returns the effective --no-auth setting. Flag takes
// precedence; otherwise the config block (and REDMINE_MCP_NO_AUTH) wins.
func resolveNoAuth(cmd *cobra.Command, flagVal bool, cfg *config.Config) bool {
	if cmd.Flags().Changed("no-auth") {
		return flagVal
	}
	if cfg != nil && cfg.MCP.NoAuth {
		return true
	}
	return flagVal
}

// httpAuthFlags carries the raw --auth-token, --auth-token-file and --no-auth
// flag values into resolveHTTPAuth.
type httpAuthFlags struct {
	Token     string
	TokenFile string
	NoAuth    bool
}

// Security warnings for the HTTP transport. They go through
// Factory.SecurityCheck, so strict mode turns authTokenFlagWarning and
// publicNoAuthWarning into errors. loopbackNoAuthWarning is only printed
// outside strict mode; strict mode generates a token instead.
const (
	authTokenFlagWarning  = "--auth-token puts the MCP bearer token in the process list and shell history; use --auth-token-file, REDMINE_MCP_AUTH_TOKEN, or mcp.auth_token instead"
	loopbackNoAuthWarning = "serving MCP over HTTP on loopback without an auth token; any local user or process that can reach the port can call the server's tools as your Redmine user. Set --auth-token-file or REDMINE_MCP_AUTH_TOKEN, or enable strict mode to generate a token"
	publicNoAuthWarning   = "serving MCP without authentication (--no-auth); anyone reachable on that interface can drive Redmine through this server"
)

// resolveHTTPAuth decides which bearer token guards the HTTP transport.
// Binds without a token get a freshly generated one (generated=true) that the
// caller prints once the listener is up, unless --no-auth opts out
// explicitly. Loopback binds are the exception: outside strict mode they stay
// open (with a warning) so local setups keep working; strict mode treats them
// like any other bind and generates a token. --no-auth on a non-loopback bind
// warns, and is refused in strict mode; on loopback it is honored silently.
func resolveHTTPAuth(f *cmdutil.Factory, cmd *cobra.Command, cfg *config.Config, isLoopback bool, flags httpAuthFlags) (token string, generated bool, err error) {
	if cmd.Flags().Changed("auth-token-file") {
		var stdin io.Reader
		if f.IOStreams != nil {
			stdin = f.IOStreams.In
		}
		token, err = readAuthTokenFile(flags.TokenFile, stdin)
		if err != nil {
			return "", false, err
		}
	} else {
		if cmd.Flags().Changed("auth-token") && flags.Token != "" {
			if err := f.SecurityCheck(cfg, authTokenFlagWarning); err != nil {
				return "", false, err
			}
		}
		token = resolveAuthToken(cmd, flags.Token, cfg)
	}
	noAuth := resolveNoAuth(cmd, flags.NoAuth, cfg)
	strict := cfg != nil && cfg.Strict

	if noAuth {
		if token != "" {
			return "", false, errors.New("--no-auth cannot be combined with an auth token (--auth-token, --auth-token-file, mcp.auth_token, or REDMINE_MCP_AUTH_TOKEN)")
		}
		if !isLoopback {
			if err := f.SecurityCheck(cfg, publicNoAuthWarning); err != nil {
				return "", false, err
			}
		}
		return "", false, nil
	}
	if token != "" {
		return token, false, nil
	}
	if isLoopback && !strict {
		if err := f.SecurityCheck(cfg, loopbackNoAuthWarning); err != nil {
			return "", false, err
		}
		return "", false, nil
	}

	token, err = generateAuthToken()
	if err != nil {
		return "", false, fmt.Errorf("generating MCP auth token: %w", err)
	}
	return token, true, nil
}

// maxAuthTokenFileSize bounds how much --auth-token-file reads, so pointing
// it at the wrong file (or an endless stream) fails fast.
const maxAuthTokenFileSize = 64 << 10

// readAuthTokenFile reads the bearer token from path, or from stdin when path
// is "-". Surrounding whitespace (such as the trailing newline of `echo` or an
// editor) is trimmed; an empty token or one containing whitespace is rejected
// because it could never match an "Authorization: Bearer <token>" header.
func readAuthTokenFile(path string, stdin io.Reader) (string, error) {
	if stdin == nil {
		stdin = os.Stdin
	}
	var (
		data []byte
		err  error
	)
	switch path {
	case "":
		return "", errors.New("--auth-token-file: file path is empty (use \"-\" to read from standard input)")
	case "-":
		data, err = io.ReadAll(io.LimitReader(stdin, maxAuthTokenFileSize+1))
	default:
		var file *os.File
		file, err = os.Open(path)
		if err == nil {
			data, err = io.ReadAll(io.LimitReader(file, maxAuthTokenFileSize+1))
			_ = file.Close()
		}
	}
	if err != nil {
		return "", fmt.Errorf("--auth-token-file: %w", err)
	}
	if len(data) > maxAuthTokenFileSize {
		return "", fmt.Errorf("--auth-token-file: %s is larger than %d bytes", authTokenSource(path), maxAuthTokenFileSize)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("--auth-token-file: %s is empty", authTokenSource(path))
	}
	if strings.ContainsAny(token, " \t\r\n") {
		return "", fmt.Errorf("--auth-token-file: %s must contain a single token without whitespace", authTokenSource(path))
	}
	return token, nil
}

func authTokenSource(path string) string {
	if path == "-" {
		return "standard input"
	}
	return path
}

// printGeneratedToken tells the operator which token clients must present.
func printGeneratedToken(w io.Writer, token string, isLoopback bool) {
	reason := "No auth token configured for a non-loopback bind"
	if isLoopback {
		reason = "No auth token configured and strict mode is enabled"
	}
	fmt.Fprintf(w,
		"%s; generated one for this run:\n\n"+
			"    %s\n\n"+
			"Clients must send \"Authorization: Bearer <token>\". Set --auth-token-file or REDMINE_MCP_AUTH_TOKEN to keep a stable token across restarts.\n",
		reason, token)
}

// listenAddr reports the configured host with the port actually bound, so
// "0.0.0.0:0" is shown as e.g. "0.0.0.0:41234" rather than "[::]:41234".
func listenAddr(configured string, bound net.Addr) string {
	host, _, err := net.SplitHostPort(configured)
	if err != nil {
		return bound.String()
	}
	_, port, err := net.SplitHostPort(bound.String())
	if err != nil {
		return bound.String()
	}
	return net.JoinHostPort(host, port)
}

// generateAuthToken returns 32 random bytes, hex-encoded.
func generateAuthToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
