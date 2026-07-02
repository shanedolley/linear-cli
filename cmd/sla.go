package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var slaCmd = &cobra.Command{
	Use:   "sla",
	Short: "Inspect SLA configuration (read-only)",
	Long: `Inspect a team's SLA rules. Each rule sets or removes an SLA when its workflow
conditions match. SLA is a Business-plan feature; on plans without it, a team
simply has no rules.

Examples:
  lincli sla list --team ENG`,
}

var slaListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List a team's SLA rules",
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		team, _ := cmd.Flags().GetString("team")
		if team == "" {
			output.Error("Team is required (--team)", plaintext, jsonOut)
			os.Exit(1)
		}

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()
		cache := newResolverCache()

		teamID, err := resolveTeam(ctx, client, cache, team)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ListSlaConfigurations(ctx, client, teamID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list SLA rules: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if len(resp.SlaConfigurations) == 0 {
			output.Info("No SLA rules configured for this team", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(resp.SlaConfigurations)
			return
		}

		headers := []string{"ID", "Name", "SLA", "Day count", "Removes"}
		rows := make([][]string, len(resp.SlaConfigurations))
		for i, r := range resp.SlaConfigurations {
			sla := ""
			if r.Sla != nil {
				sla = fmt.Sprintf("%.0f", *r.Sla)
			}
			dayCount := ""
			if r.SlaType != nil {
				dayCount = string(*r.SlaType)
			}
			rows[i] = []string{
				r.Id,
				truncateString(r.Name, 35),
				sla,
				dayCount,
				fmt.Sprintf("%t", r.RemovesSla),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d rules\n", color.New(color.FgGreen).Sprint("✓"), len(resp.SlaConfigurations))
		}
	},
}

func init() {
	rootCmd.AddCommand(slaCmd)
	slaCmd.AddCommand(slaListCmd)

	slaListCmd.Flags().StringP("team", "t", "", "Team key or ID (required)")
}
