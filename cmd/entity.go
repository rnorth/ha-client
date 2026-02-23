package cmd

import (
	"os"

	"github.com/rnorth/ha-client/internal/output"
	"github.com/spf13/cobra"
)

var entityCmd = &cobra.Command{Use: "entity", Short: "Manage the entity registry"}

var entityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered entities",
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		entities, err := wsc.ListEntities()
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), entities, []string{"EntityID", "Name", "Platform", "AreaID"})
	},
}

var entityGetCmd = &cobra.Command{
	Use:   "get <entity_id>",
	Short: "Get an entity from the registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		entity, err := wsc.GetEntity(args[0])
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), entity, nil)
	},
}

var entityDescribeCmd = &cobra.Command{
	Use:   "describe <entity_id>",
	Short: "Show full entity registry details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		entity, err := wsc.GetEntity(args[0])
		if err != nil {
			return err
		}
		format := resolveFormat()
		if format == output.FormatTable {
			format = output.FormatYAML
		}
		return output.Render(os.Stdout, format, entity, nil)
	},
}

func init() {
	entityCmd.AddCommand(entityListCmd, entityGetCmd, entityDescribeCmd)
	rootCmd.AddCommand(entityCmd)
}
