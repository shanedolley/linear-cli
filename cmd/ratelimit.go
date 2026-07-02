package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rateLimitCmd = &cobra.Command{
	Use:     "rate-limit",
	Aliases: []string{"ratelimit"},
	Short:   "Show the current API rate-limit budget",
	Long: `Show the authenticated client's live rate-limit budget from Linear's
rateLimitStatus query: the remaining and allowed quota, and when each limit
resets, for every limit type (requests and query complexity).

Examples:
  lincli rate-limit
  lincli rate-limit --json`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		resp, err := api.RateLimitStatus(context.Background(), client)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get rate-limit status: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		status := resp.RateLimitStatus
		if jsonOut {
			output.JSON(status)
			return
		}

		headers := []string{"Type", "Remaining", "Allowed", "Resets in"}
		rows := make([][]string, len(status.Limits))
		for i, l := range status.Limits {
			rows[i] = []string{
				l.Type,
				fmt.Sprintf("%.0f", l.RemainingAmount),
				fmt.Sprintf("%.0f", l.AllowedAmount),
				formatResetIn(l.Reset),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %s (%s)\n", color.New(color.FgGreen).Sprint("✓"), status.Kind, derefStr(status.Identifier))
		}
	},
}

// formatResetIn renders a UNIX reset timestamp (milliseconds) as a short
// human-readable "resets in" duration relative to now, e.g. "42m" or "1h3m".
// A reset already in the past renders as "now".
func formatResetIn(resetMillis float64) string {
	reset := time.UnixMilli(int64(resetMillis))
	d := time.Until(reset).Round(time.Minute)
	if d <= 0 {
		return "now"
	}
	return d.String()
}

func init() {
	rootCmd.AddCommand(rateLimitCmd)
}
