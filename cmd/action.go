package cmd

import (
	"encoding/json"
	"fmt"
	"os"

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

var actionDataJSON string

var actionCallCmd = &cobra.Command{
	Use:   "call <domain.action>",
	Short: "Call a Home Assistant action",
	Long:  "Call a Home Assistant action. Example: ha-client action call light.turn_on --data '{\"entity_id\": \"light.desk\"}'",
	Args:  cobra.ExactArgs(1),
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

		var data map[string]interface{}
		if actionDataJSON != "" {
			if err := json.Unmarshal([]byte(actionDataJSON), &data); err != nil {
				return fmt.Errorf("invalid --data JSON: %w", err)
			}
		}

		if err := c.CallAction(parts[0], parts[1], data); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Action called successfully.")
		return nil
	},
}

func splitDomainAction(s string) []string {
	for i, c := range s {
		if c == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return nil
}

func init() {
	actionCallCmd.Flags().StringVar(&actionDataJSON, "data", "", "JSON data to pass to the action")
	actionCmd.AddCommand(actionListCmd, actionCallCmd)
	rootCmd.AddCommand(actionCmd)
}
