package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/rnorth/ha-client/internal/client"
	"github.com/rnorth/ha-client/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var automationCmd = &cobra.Command{Use: "automation", Short: "Manage Home Assistant automations"}

var automationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all automations",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c := client.NewRESTClient(cfg.Server, cfg.Token)
		states, err := c.ListStates()
		if err != nil {
			return err
		}
		type row struct {
			EntityID     string `json:"entity_id" yaml:"entity_id"`
			FriendlyName string `json:"friendly_name" yaml:"friendly_name"`
			State        string `json:"state" yaml:"state"`
		}
		var rows []row
		for _, s := range states {
			if !strings.HasPrefix(s.EntityID, "automation.") {
				continue
			}
			name, _ := s.Attributes["friendly_name"].(string)
			rows = append(rows, row{EntityID: s.EntityID, FriendlyName: name, State: s.State})
		}
		return output.Render(os.Stdout, resolveFormat(), rows, nil)
	},
}

var automationGetCmd = &cobra.Command{
	Use:   "get <entity_id>",
	Short: "Get automation state",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c := client.NewRESTClient(cfg.Server, cfg.Token)
		state, err := c.GetState(automationID(args[0]))
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), state, nil)
	},
}

var automationDescribeCmd = &cobra.Command{
	Use:   "describe <entity_id>",
	Short: "Show full automation details including attributes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c := client.NewRESTClient(cfg.Server, cfg.Token)
		state, err := c.GetState(automationID(args[0]))
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveDescribeFormat(), state, nil)
	},
}

var automationExportCmd = &cobra.Command{
	Use:   "export <entity_id>",
	Short: "Export automation config as YAML",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()

		entityID := automationID(args[0])
		cfg, err := wsc.GetAutomationConfig(entityID)
		if err != nil {
			return err
		}

		return output.Render(cmd.OutOrStdout(), resolveDescribeFormat(), cfg, nil)
	},
}

// automationID ensures the entity ID has the "automation." prefix.
func automationID(s string) string {
	if strings.HasPrefix(s, "automation.") {
		return s
	}
	return "automation." + s
}

func automationAction(action string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c := client.NewRESTClient(cfg.Server, cfg.Token)
		id := automationID(args[0])
		if err := c.CallAction("automation", action, map[string]interface{}{"entity_id": id}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "automation.%s called for %s\n", action, id)
		return nil
	}
}

var automationApplyFile string
var automationApplyDryRun bool

var automationApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply (create or update) an automation from a YAML file",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(automationApplyFile)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		var cfg map[string]interface{}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parsing YAML: %w", err)
		}

		idVal, ok := cfg["id"]
		if !ok || idVal == nil || idVal == "" {
			return fmt.Errorf("automation YAML must contain an 'id' field")
		}
		autoID, ok := idVal.(string)
		if !ok || autoID == "" {
			return fmt.Errorf("automation 'id' field must be a non-empty string")
		}

		haConfig, err := resolveConfig()
		if err != nil {
			return err
		}
		rc := client.NewRESTClient(haConfig.Server, haConfig.Token)

		if automationApplyDryRun {
			return runDryRun(cmd, rc, autoID, cfg)
		}

		if err := rc.SaveAutomationConfig(autoID, cfg); err != nil {
			return fmt.Errorf("apply failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "automation %q applied\n", autoID)
		return nil
	},
}

func runDryRun(cmd *cobra.Command, rc interface {
	GetAutomationConfig(string) (map[string]interface{}, error)
}, autoID string, newCfg map[string]interface{}) error {
	newYAML, err := yaml.Marshal(newCfg)
	if err != nil {
		return err
	}

	current, err := rc.GetAutomationConfig(autoID)
	var oldYAML []byte
	if err != nil {
		// RESTClient returns a plain string error for HTTP 404s; there is no
		// dedicated error type, so we match on the message text.
		if strings.Contains(err.Error(), "not found") {
			// Automation doesn't exist yet — treat as all-new
			oldYAML = []byte{}
		} else {
			return fmt.Errorf("loading current automation config: %w", err)
		}
	} else {
		oldYAML, err = yaml.Marshal(current)
		if err != nil {
			return err
		}
	}

	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(oldYAML)),
		B:        difflib.SplitLines(string(newYAML)),
		FromFile: "current",
		ToFile:   "new",
		Context:  3,
	}
	text, err := difflib.GetUnifiedDiffString(ud)
	if err != nil {
		return err
	}
	if text == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "(no changes)")
		return nil
	}
	// Strip the unchanged context lines (those that start with a space) to keep
	// the dry-run output compact; the @@ hunk headers already tell the reader
	// where in the file each change lives.
	var sb strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "@") {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
	fmt.Fprint(cmd.OutOrStdout(), sb.String())
	return nil
}

func init() {
	automationCmd.AddCommand(
		automationListCmd,
		automationGetCmd,
		automationDescribeCmd,
		automationExportCmd,
		automationApplyCmd,
		&cobra.Command{Use: "trigger <entity_id>", Short: "Trigger an automation", Args: cobra.ExactArgs(1), RunE: automationAction("trigger")},
		&cobra.Command{Use: "enable <entity_id>", Short: "Enable an automation", Args: cobra.ExactArgs(1), RunE: automationAction("turn_on")},
		&cobra.Command{Use: "disable <entity_id>", Short: "Disable an automation", Args: cobra.ExactArgs(1), RunE: automationAction("turn_off")},
	)
	automationApplyCmd.Flags().StringVarP(&automationApplyFile, "filename", "f", "", "path to automation YAML file (required)")
	_ = automationApplyCmd.MarkFlagRequired("filename")
	automationApplyCmd.Flags().BoolVar(&automationApplyDryRun, "dry-run", false, "print diff without applying")
	rootCmd.AddCommand(automationCmd)
}
