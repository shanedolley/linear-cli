package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		team, _ := cmd.Flags().GetString("team")
		if team == "" {
			return errors.New("Team is required (--team)")
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		teamID, err := resolveTeam(ctx, client, cache, team)
		if err != nil {
			return err
		}

		resp, err := api.ListSlaConfigurations(ctx, client, teamID)
		if err != nil {
			return fmt.Errorf("Failed to list SLA rules: %w", err)
		}

		if len(resp.SlaConfigurations) == 0 {
			output.Info("No SLA rules configured for this team", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.SlaConfigurations)
			return nil
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
		return nil
	},
}

func init() {
	rootCmd.AddCommand(slaCmd)
	slaCmd.AddCommand(slaListCmd)

	slaListCmd.Flags().StringP("team", "t", "", "Team key or ID (required)")
}
