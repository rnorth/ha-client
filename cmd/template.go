package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Work with Home Assistant Jinja templates",
}

var templateEvalFile string

var templateEvalCmd = &cobra.Command{
	Use:   "eval [template | -]",
	Short: "Evaluate a Jinja template",
	Long: `Evaluate a Jinja template server-side and print the result.

Examples:
  ha-client template eval '{{ states("sensor.temperature") }}'
  echo '{{ states("sensor.temperature") }}' | ha-client template eval -
  ha-client template eval -f template.j2`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var tmpl string

		switch {
		case templateEvalFile != "":
			data, err := os.ReadFile(templateEvalFile)
			if err != nil {
				return fmt.Errorf("reading template file: %w", err)
			}
			tmpl = string(data)
		case len(args) == 1 && args[0] == "-":
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			tmpl = string(data)
		case len(args) == 1:
			tmpl = args[0]
		default:
			return fmt.Errorf("provide a template as an argument, via stdin (-), or with --file")
		}

		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c := client.NewRESTClient(cfg.Server, cfg.Token)

		result, err := c.RenderTemplate(tmpl)
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), result)
		return nil
	},
}

func init() {
	templateEvalCmd.Flags().StringVarP(&templateEvalFile, "file", "f", "", "read template from file")
	templateCmd.AddCommand(templateEvalCmd)
	rootCmd.AddCommand(templateCmd)
}
