package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/rnorth/ha-client/internal/config"
	clierrors "github.com/rnorth/ha-client/internal/errors"
	"github.com/rnorth/ha-client/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	outputFormat string
	serverFlag   string
	tokenFlag    string
)

var rootCmd = &cobra.Command{
	Use:   "ha-client",
	Short: "A kubectl-style CLI for Home Assistant",
	Long:  "Interact with Home Assistant instances from the command line. Designed for humans and AI agents.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		ce := clierrors.Classify(err)

		if !term.IsTerminal(int(os.Stderr.Fd())) {
			// Structured JSON error for agents/pipes
			errObj := map[string]string{"error": ce.Error(), "code": ce.Code}
			data, _ := json.Marshal(errObj)
			fmt.Fprintln(os.Stderr, string(data))
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(ce.ExitCode)
	}
}

func resolveConfig() (*config.Config, error) {
	cfg, err := config.Resolve(serverFlag, tokenFlag)
	if err != nil {
		return nil, err
	}
	return cfg, cfg.Validate()
}

func resolveFormat() output.Format {
	return output.DetectFormat(outputFormat, os.Stdout)
}

// resolveDescribeFormat returns the output format for "describe" subcommands.
// Describe commands expose deeply-nested data (attributes, config blocks) that
// does not render usefully as a flat table, so we upgrade table → YAML at a TTY.
// YAML is preferred over JSON for human-facing output because it is less noisy
// (no quotes, no braces) and easier to scan at a glance.
func resolveDescribeFormat() output.Format {
	format := resolveFormat()
	if format == output.FormatTable {
		return output.FormatYAML
	}
	return format
}

func newWSClient() (*client.WSClient, error) {
	cfg, err := resolveConfig()
	if err != nil {
		return nil, err
	}
	return client.NewWSClient(cfg.Server, cfg.Token)
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "output format: table, json, yaml (default: auto-detect TTY)")
	rootCmd.PersistentFlags().StringVar(&serverFlag, "server", "", "HA server URL (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&tokenFlag, "token", "", "HA access token (overrides config/env)")
	rootCmd.Version = "0.1.0"
}
