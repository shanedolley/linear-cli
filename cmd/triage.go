package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Triage responsibilities are read-only here (list, get). Writes
// (triageResponsibilityCreate/Update/Delete) require Linear's Business plan -
// the API rejects them with FORBIDDEN "Subscribe to the Business plan to access
// triage responsibility", even on a triage-enabled team. The write commands are
// deferred per the plan-availability gate; only the available read path ships.

var triageCmd = &cobra.Command{
	Use:   "triage",
	Short: "View triage responsibilities",
	Long: `View triage responsibilities: who handles newly triaged issues for a team,
either a manual set of users or a time schedule (on-call rotation).

Read-only: creating and editing triage responsibilities requires Linear's
Business plan, so those commands are not available.

Examples:
  lincli triage list
  lincli triage get <id>`,
}

var triageListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List triage responsibilities",
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := triageClient()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		resp, err := api.ListTriageResponsibilities(ctx, client, limitPtr, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list triage responsibilities: %v", err)
		}

		if resp.TriageResponsibilities == nil || len(resp.TriageResponsibilities.Nodes) == 0 {
			output.Info("No triage responsibilities found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.TriageResponsibilities.Nodes)
			return nil
		}

		headers := []string{"ID", "Team", "Action", "Schedule", "Current"}
		rows := make([][]string, len(resp.TriageResponsibilities.Nodes))
		for i, node := range resp.TriageResponsibilities.Nodes {
			f := node.TriageResponsibilityFields
			team := "-"
			if f.Team != nil {
				team = f.Team.Key
			}
			schedule := "-"
			if f.TimeSchedule != nil {
				schedule = f.TimeSchedule.Name
			}
			current := "-"
			if f.CurrentUser != nil {
				current = f.CurrentUser.Name
			}
			rows[i] = []string{f.Id, team, string(f.Action), truncateString(schedule, 20), truncateString(current, 20)}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d responsibilities\n", color.New(color.FgGreen).Sprint("✓"), len(resp.TriageResponsibilities.Nodes))
		}
		return nil
	},
}

var triageGetCmd = &cobra.Command{
	Use:     "get <id>",
	Aliases: []string{"show"},
	Short:   "Get a triage responsibility",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut := viper.GetBool("json")

		client, ctx, err := triageClient()
		if err != nil {
			return err
		}

		resp, err := api.GetTriageResponsibility(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to get triage responsibility: %v", err)
		}
		if resp.TriageResponsibility == nil {
			return fmt.Errorf("triage responsibility %q not found", args[0])
		}
		f := resp.TriageResponsibility.TriageResponsibilityFields

		if jsonOut {
			output.JSON(resp.TriageResponsibility)
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Triage responsibility:"), f.Id)
		if f.Team != nil {
			fmt.Printf("  Team:     %s (%s)\n", f.Team.Name, f.Team.Key)
		}
		fmt.Printf("  Action:   %s\n", string(f.Action))
		if f.TimeSchedule != nil {
			fmt.Printf("  Schedule: %s\n", f.TimeSchedule.Name)
		}
		if f.CurrentUser != nil {
			fmt.Printf("  Current:  %s\n", f.CurrentUser.Name)
		}
		if f.ManualSelection != nil && len(f.ManualSelection.UserIds) > 0 {
			fmt.Printf("  Users:    %s\n", strings.Join(f.ManualSelection.UserIds, ", "))
		}
		return nil
	},
}

// triageClient builds the authenticated client and context shared by the triage
// subcommands.
func triageClient() (graphql.Client, context.Context, error) {
	client, err := newGraphQLClient()
	if err != nil {
		return nil, nil, err
	}
	return client, context.Background(), nil
}

func init() {
	rootCmd.AddCommand(triageCmd)

	triageCmd.AddCommand(triageListCmd)
	triageCmd.AddCommand(triageGetCmd)

	triageListCmd.Flags().IntP("limit", "l", 50, "Maximum number of responsibilities to fetch")
}
