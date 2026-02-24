package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/rnorth/ha-client/internal/output"
	"github.com/spf13/cobra"
)

var actionCmd = &cobra.Command{
	Use:   "action",
	Short: "Manage Home Assistant actions (formerly services)",
}

var actionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available actions",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c := client.NewRESTClient(cfg.Server, cfg.Token)
		domains, err := c.ListActions()
		if err != nil {
			return err
		}
		// Flatten to a list of "domain.action" rows for table display
		type row struct {
			Action      string `json:"action" yaml:"action"`
			Description string `json:"description" yaml:"description"`
		}
		var rows []row
		for _, d := range domains {
			for name, detail := range d.Services {
				rows = append(rows, row{
					Action:      d.Domain + "." + name,
					Description: detail.Description,
				})
			}
		}
		return output.Render(os.Stdout, resolveFormat(), rows, nil)
	},
}

var (
	actionDataJSONRaw string
	actionDataFields  []string
	actionEntityID    string
)

var actionCallCmd = &cobra.Command{
	Use:   "call <domain.action>",
	Short: "Call a Home Assistant action",
	Long: `Call a Home Assistant action.

Examples:
  ha-client action call light.turn_on --entity_id=light.desk
  ha-client action call light.turn_on --entity_id=light.desk -d transition=5 -d brightness_pct=80
  ha-client action call light.turn_on --data-json '{"entity_id":"light.desk","effect":"rainbow"}'
  ha-client action call light.turn_on --data-json '{"transition":5}' -d brightness_pct=80 --entity_id=light.desk`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c := client.NewRESTClient(cfg.Server, cfg.Token)

		// Parse "domain.action"
		parts := splitDomainAction(args[0])
		if parts == nil {
			return fmt.Errorf("invalid action format %q: expected domain.action (e.g. light.turn_on)", args[0])
		}

		data, err := buildActionData(actionDataJSONRaw, actionDataFields, actionEntityID)
		if err != nil {
			return err
		}

		if err := c.CallAction(parts[0], parts[1], data); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Action called successfully.")
		return nil
	},
}

// splitDomainAction splits "domain.action" at the first dot, returning
// [domain, action]. SplitN with n=2 means action names that contain dots
// (e.g. a script named "run.scene.on") are kept intact in the second element.
func splitDomainAction(s string) []string {
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil
	}
	return parts
}

// buildActionData merges the three flag sources into a single data map.
// Merge order (later wins): --data-json < -d fields < --entity_id.
func buildActionData(dataJSON string, fields []string, entityID string) (map[string]interface{}, error) {
	data := map[string]interface{}{}

	if dataJSON != "" {
		if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
			return nil, fmt.Errorf("invalid --data-json: %w", err)
		}
	}

	for _, f := range fields {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			return nil, fmt.Errorf("invalid -d flag %q: expected key=value", f)
		}
		data[k] = v
	}

	if entityID != "" {
		data["entity_id"] = entityID
	}

	if len(data) == 0 {
		return nil, nil
	}
	return data, nil
}

func init() {
	actionCallCmd.Flags().StringVar(&actionDataJSONRaw, "data-json", "", "raw JSON data payload")
	actionCallCmd.Flags().StringArrayVarP(&actionDataFields, "data", "d", nil, "data field as key=value (repeatable)")
	actionCallCmd.Flags().StringVar(&actionEntityID, "entity_id", "", "entity ID to target (shorthand for -d entity_id=...)")
	actionCmd.AddCommand(actionListCmd, actionCallCmd)
	rootCmd.AddCommand(actionCmd)
}
