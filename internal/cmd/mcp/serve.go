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
	"strings"

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
			"flags) selects the Redmine instance. When nothing is configured yet, " +
			"the stdio server still starts, without Redmine tools, and tells the " +
			"agent how to finish the setup (with --http it exits with the error " +
			"instead).\n\n" +
			"By default only read tools are registered. Pass --enable-writes to " +
			"also register create/update/delete tools. Use --http to listen on " +
			"an HTTP address such as :8080 instead of stdio. The HTTP transport " +
			"defaults to a loopback bind (127.0.0.1); pass an explicit host to " +
			"listen elsewhere. When a token is configured (--auth-token-file, " +
			"REDMINE_MCP_AUTH_TOKEN or the profile's mcp.auth_token), every " +
			"request must carry it as a bearer token, on any bind. On a " +
			"non-loopback bind without a configured token, a random token is " +
			"generated at startup and printed to stderr. A loopback bind " +
			"without a token stays open to every local user and process; the " +
			"server warns about it. Pass --no-auth only when something in front " +
			"of the server (e.g. a reverse proxy) already authenticates " +
			"clients.\n\n" +
			"To narrow the set of tools exposed to MCP clients, use " +
			"--enable-groups / --disable-groups (allow- and deny-list of tool " +
			"groups) and --enable-tools / --disable-tools (per-tool overrides). " +
			"Run `redmine mcp tools` to see the available groups and tools.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			version := cmd.Root().Version
			if version == "" {
				version = "dev"
			}

			cfg, cfgErr := f.Config()
			// Filter flags are the user's explicit input, so invalid ones fail
			// even when the server runs unconfigured.
			spec, err := buildFilterSpec(cfg, cmd, enableGroups, disableGroups, enableTools, disableTools)
			if err != nil {
				return err
			}

			// main cancels this context on the first SIGINT/SIGTERM, which
			// shuts both transports down cleanly, and restores default
			// signal handling so a second Ctrl-C still kills the process.
			// Serve must not catch signals itself, or it would swallow that
			// second Ctrl-C while shutting down.
			ctx := cmdutil.Context(cmd)

			client, err := f.ApiClient()
			if err != nil {
				// A host such as Claude Code only reports "Failed to connect"
				// when the process exits before the handshake, so a missing
				// setup keeps the server up and tells the agent what to do.
				// Over HTTP there is no host to surface that: the operator
				// reads the log and a restart is needed anyway, so fail.
				// Errors from explicit input (an unknown --profile, a broken
				// config file) fail too instead of masking the typo.
				if !isSetupProblem(cfg, cfgErr) || httpRequested(cmd, httpAddr, cfg, cfgErr) {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "redmine mcp: Redmine is not configured (%v). Serving without Redmine tools; run 'redmine auth login' (or set REDMINE_SERVER and REDMINE_API_KEY), then restart the MCP server.\n", err)
				srv := mcpserver.BuildSetupServer(err.Error(), mcpserver.Options{Name: name, Version: version})
				return srv.Run(ctx, &sdk.StdioTransport{})
			}

			opts := mcpserver.Options{
				EnableWrites: resolveEnableWrites(cmd, enableWrites, cfg),
				Name:         name,
				Version:      version,
			}
			opts = mcpserver.ResolveOptions(opts, spec)

			startCredentialCheck(ctx, f, cfg, client)

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
					printGeneratedToken(cmd.ErrOrStderr(), token)
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
	cmd.Flags().StringVar(&authToken, "auth-token", "", "Require this bearer token in the Authorization header for every HTTP request. When no token is set, one is generated at startup for an --http bind outside of loopback. Visible in the process list; prefer --auth-token-file, REDMINE_MCP_AUTH_TOKEN, or mcp.auth_token.")
	cmd.Flags().StringVar(&authTokenFile, "auth-token-file", "", "Read the --auth-token value from a file, or from standard input with \"-\", so it never appears in the process list.")
	cmd.MarkFlagsMutuallyExclusive("auth-token", "auth-token-file")
	cmd.Flags().BoolVar(&noAuth, "no-auth", false, "Serve a non-loopback --http bind without a bearer token instead of generating one. Only use behind an authenticating proxy.")
	cmd.Flags().StringVar(&name, "name", "redmine-cli", "Server name advertised to MCP clients")
	cmd.Flags().StringSliceVar(&enableGroups, "enable-groups", nil, "Comma-separated tool groups to expose (default: all). See redmine mcp tools.")
	cmd.Flags().StringSliceVar(&disableGroups, "disable-groups", nil, "Comma-separated tool groups to hide. Applied after --enable-groups.")
	cmd.Flags().StringSliceVar(&enableTools, "enable-tools", nil, "Allow-list of tool names. Tools outside the list are hidden.")
	cmd.Flags().StringSliceVar(&disableTools, "disable-tools", nil, "Deny-list of tool names. Applied after --enable-tools.")

	cmdutil.MarkNoJSONOutput(cmd)
	return cmd
}

// isSetupProblem reports whether the failure to build a client means Redmine
// is not set up yet (no server, no usable stored credential, no profile
// selected) rather than a mistake in explicit input. cfgErr is the error from
// loading the configuration; cfg is nil when it failed.
func isSetupProblem(cfg *config.Config, cfgErr error) bool {
	if cfgErr != nil {
		return config.IsKeyringSecretError(cfgErr) || config.IsNoActiveProfileError(cfgErr)
	}
	return cfg != nil && cfg.Server == ""
}

// httpRequested reports whether HTTP mode was asked for, even when the config
// could not be loaded (cfg is nil then, so REDMINE_MCP_HTTP and a keyring
// profile's mcp.http are checked separately). An explicit empty --http means
// stdio.
func httpRequested(cmd *cobra.Command, flagVal string, cfg *config.Config, cfgErr error) bool {
	if cmd.Flags().Changed("http") {
		return flagVal != ""
	}
	if cfg != nil {
		return cfg.MCP.HTTP != ""
	}
	return os.Getenv("REDMINE_MCP_HTTP") != "" || config.KeyringSecretMCPHTTP(cfgErr) != ""
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

// Security warnings for the HTTP transport, printed through Factory.Warn.
const (
	loopbackNoAuthWarning = "serving MCP over HTTP on loopback without an auth token; any local user or process that can reach the port can call the server's tools as your Redmine user. Set --auth-token-file or REDMINE_MCP_AUTH_TOKEN to require a token"
	publicNoAuthWarning   = "serving MCP without authentication (--no-auth); anyone reachable on that interface can drive Redmine through this server"
)

// resolveHTTPAuth decides which bearer token guards the HTTP transport.
// Non-loopback binds without a token get a freshly generated one
// (generated=true) that the caller prints once the listener is up, unless
// --no-auth opts out explicitly, which warns. Loopback binds without a token
// stay open with a warning so local setups keep working; --no-auth on
// loopback is honored silently.
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
		token = resolveAuthToken(cmd, flags.Token, cfg)
		if token == "" && !cmd.Flags().Changed("auth-token") {
			// A keyring profile keeps mcp.auth_token in the keyring, and
			// config loading leaves it there until it is needed. If the
			// keyring can't be read, a saved token may exist, so refuse to
			// serve without it unless --no-auth asks for exactly that.
			token, err = config.ResolveMCPAuthToken(cfg, f.DebugLogger())
			if err != nil && !resolveNoAuth(cmd, flags.NoAuth, cfg) {
				return "", false, fmt.Errorf("%w; refusing to serve without a token that may exist. Set REDMINE_MCP_AUTH_TOKEN or --auth-token-file, or pass --no-auth to serve without authentication", err)
			}
		}
	}
	noAuth := resolveNoAuth(cmd, flags.NoAuth, cfg)

	if noAuth {
		if token != "" {
			return "", false, errors.New("--no-auth cannot be combined with an auth token (--auth-token, --auth-token-file, mcp.auth_token, or REDMINE_MCP_AUTH_TOKEN)")
		}
		if !isLoopback {
			f.Warn(publicNoAuthWarning)
		}
		return "", false, nil
	}
	if token != "" {
		return token, false, nil
	}
	if isLoopback {
		f.Warn(loopbackNoAuthWarning)
		return "", false, nil
	}

	token, err = generateAuthToken()
	if err != nil {
		return "", false, fmt.Errorf("generating MCP auth token: %w", err)
	}
	return token, true, nil
}

// readAuthTokenFile reads the bearer token from path, or from stdin when path
// is "-" (os.Stdin when stdin is nil). A token containing whitespace is
// rejected because it could never match an "Authorization: Bearer <token>"
// header.
func readAuthTokenFile(path string, stdin io.Reader) (string, error) {
	if stdin == nil {
		stdin = os.Stdin
	}
	token, err := cmdutil.ReadSecretFile("--auth-token-file", path, stdin)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(token, " \t\r\n") {
		return "", errors.New("--auth-token-file: the token must not contain whitespace")
	}
	return token, nil
}

// printGeneratedToken tells the operator which token clients must present.
func printGeneratedToken(w io.Writer, token string) {
	fmt.Fprintf(w,
		"No auth token configured for a non-loopback bind; generated one for this run:\n\n"+
			"    %s\n\n"+
			"Clients must send \"Authorization: Bearer <token>\". Set --auth-token-file or REDMINE_MCP_AUTH_TOKEN to keep a stable token across restarts.\n",
		token)
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
