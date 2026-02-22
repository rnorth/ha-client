package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rnorth/ha-cli/internal/client"
	"github.com/spf13/cobra"
)

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "Subscribe to Home Assistant events",
}

var eventTypeFilter string

var eventWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Stream events in real-time (Ctrl+C to stop)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}

		wsc, err := client.NewWSClient(cfg.Server, cfg.Token)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer wsc.Close()

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		done := make(chan error, 1)

		go func() {
			done <- wsc.SubscribeEvents(eventTypeFilter, func(event json.RawMessage) bool {
				select {
				case <-stop:
					return false
				default:
				}
				var pretty map[string]interface{}
				if json.Unmarshal(event, &pretty) == nil {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					_ = enc.Encode(pretty)
				}
				return true
			})
		}()

		select {
		case <-stop:
			fmt.Fprintln(os.Stderr, "\nStopped.")
		case err := <-done:
			if err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	eventWatchCmd.Flags().StringVar(&eventTypeFilter, "type", "", "filter to a specific event type (e.g. state_changed)")
	eventCmd.AddCommand(eventWatchCmd)
	rootCmd.AddCommand(eventCmd)
}
