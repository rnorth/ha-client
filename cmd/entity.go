package cmd

import (
	"os"
	"strings"

	"github.com/rnorth/ha-client/internal/output"
	"github.com/spf13/cobra"
)

var entityCmd = &cobra.Command{Use: "entity", Short: "Manage the entity registry"}

var entityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered entities",
	Long: `List all registered entities.

Examples:
  ha-client entity list
  ha-client entity list -o json | jq '.[] | select(.platform == "hue")'`,
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
		if entityListDomain != "" {
			prefix := entityListDomain + "."
			filtered := entities[:0]
			for _, e := range entities {
				if strings.HasPrefix(e.EntityID, prefix) {
					filtered = append(filtered, e)
				}
			}
			entities = filtered
		}
		return output.Render(os.Stdout, resolveFormat(), entities, []string{"EntityID", "Name", "Platform", "AreaID"}, renderOpts()...)
	},
}

var entityGetCmd = &cobra.Command{
	Use:   "get <entity_id>",
	Short: "Get an entity from the registry",
	Long: `Get details for a specific entity from the registry.

Examples:
  ha-client entity get light.desk
  ha-client entity get light.desk -o json`,
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
		return output.Render(os.Stdout, resolveFormat(), entity, nil, renderOpts()...)
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
		return output.Render(os.Stdout, resolveDescribeFormat(), entity, nil, renderOpts()...)
	},
}

var entityListDomain string

func init() {
	entityListCmd.Flags().StringVar(&entityListDomain, "domain", "", "filter by entity domain (e.g. light, sensor)")
	entityCmd.AddCommand(entityListCmd, entityGetCmd, entityDescribeCmd)
	rootCmd.AddCommand(entityCmd)
}
