package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rnorth/ha-cli/internal/client"
	"github.com/rnorth/ha-cli/internal/output"
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage entity states",
}

var stateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all entity states",
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
		return output.Render(os.Stdout, resolveFormat(), states, []string{"EntityID", "State", "LastUpdated"})
	},
}

var stateGetCmd = &cobra.Command{
	Use:   "get <entity_id>",
	Short: "Get state of an entity",
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
		state, err := c.GetState(args[0])
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), state, nil)
	},
}

var stateDescribeCmd = &cobra.Command{
	Use:   "describe <entity_id>",
	Short: "Show full state and attributes of an entity",
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
		state, err := c.GetState(args[0])
		if err != nil {
			return err
		}
		// Always render describe as JSON/YAML (attributes map doesn't render well in table)
		format := resolveFormat()
		if format == output.FormatTable {
			format = output.FormatYAML
		}
		return output.Render(os.Stdout, format, state, nil)
	},
}

var stateSetCmd = &cobra.Command{
	Use:   "set <entity_id> <state>",
	Short: "Set the state of an entity",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		var attrs map[string]interface{}
		if attrJSON != "" {
			if err := json.Unmarshal([]byte(attrJSON), &attrs); err != nil {
				return fmt.Errorf("invalid --attributes JSON: %w", err)
			}
		}
		state, err := c.SetState(args[0], args[1], attrs)
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), state, nil)
	},
}

var attrJSON string

func init() {
	stateSetCmd.Flags().StringVar(&attrJSON, "attributes", "", "JSON attributes to set alongside the state")
	stateCmd.AddCommand(stateListCmd, stateGetCmd, stateDescribeCmd, stateSetCmd)
	rootCmd.AddCommand(stateCmd)
}
