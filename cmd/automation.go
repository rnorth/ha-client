package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/rnorth/ha-cli/internal/client"
	"github.com/rnorth/ha-cli/internal/output"
	"github.com/spf13/cobra"
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
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
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
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
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
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		state, err := c.GetState(automationID(args[0]))
		if err != nil {
			return err
		}
		format := resolveFormat()
		if format == output.FormatTable {
			format = output.FormatYAML
		}
		return output.Render(os.Stdout, format, state, nil)
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
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		id := automationID(args[0])
		if err := c.CallAction("automation", action, map[string]interface{}{"entity_id": id}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "automation.%s called for %s\n", action, id)
		return nil
	}
}

func init() {
	automationCmd.AddCommand(
		automationListCmd,
		automationGetCmd,
		automationDescribeCmd,
		&cobra.Command{Use: "trigger <entity_id>", Short: "Trigger an automation", Args: cobra.ExactArgs(1), RunE: automationAction("trigger")},
		&cobra.Command{Use: "enable <entity_id>", Short: "Enable an automation", Args: cobra.ExactArgs(1), RunE: automationAction("turn_on")},
		&cobra.Command{Use: "disable <entity_id>", Short: "Disable an automation", Args: cobra.ExactArgs(1), RunE: automationAction("turn_off")},
	)
	rootCmd.AddCommand(automationCmd)
}
