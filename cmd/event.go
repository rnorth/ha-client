package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rnorth/ha-client/internal/client"
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
		// Buffered so the goroutine below can send without blocking if we already
		// returned via the signal branch.
		done := make(chan error, 1)

		// Run SubscribeEvents in a goroutine because it blocks indefinitely.
		// The main goroutine then selects between a user interrupt and a connection
		// error, allowing a clean shutdown on Ctrl+C.
		go func() {
			done <- wsc.SubscribeEvents(eventTypeFilter, func(event json.RawMessage) bool {
				// Check for stop signal before printing each event so we don't
				// emit a partial event after the user has asked us to quit.
				select {
				case <-stop:
					return false
				default:
				}
				var pretty map[string]interface{}
				// Best-effort pretty-print; if the event isn't valid JSON we skip it
				// silently rather than crashing — HA occasionally sends non-JSON events.
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
