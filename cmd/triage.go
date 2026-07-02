package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
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
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx := triageClient(plaintext, jsonOut)

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		resp, err := api.ListTriageResponsibilities(ctx, client, limitPtr, nil, nil)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list triage responsibilities: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if resp.TriageResponsibilities == nil || len(resp.TriageResponsibilities.Nodes) == 0 {
			output.Info("No triage responsibilities found", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(resp.TriageResponsibilities.Nodes)
			return
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
	},
}

var triageGetCmd = &cobra.Command{
	Use:     "get <id>",
	Aliases: []string{"show"},
	Short:   "Get a triage responsibility",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx := triageClient(plaintext, jsonOut)

		resp, err := api.GetTriageResponsibility(ctx, client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get triage responsibility: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		f := resp.TriageResponsibility.TriageResponsibilityFields

		if jsonOut {
			output.JSON(resp.TriageResponsibility)
			return
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
	},
}

// triageClient builds the authenticated client and context shared by the triage
// subcommands, exiting on an auth failure.
func triageClient(plaintext, jsonOut bool) (*api.Client, context.Context) {
	authHeader, err := auth.GetAuthHeader()
	if err != nil {
		output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
		os.Exit(1)
	}
	return api.NewClient(authHeader), context.Background()
}

func init() {
	rootCmd.AddCommand(triageCmd)

	triageCmd.AddCommand(triageListCmd)
	triageCmd.AddCommand(triageGetCmd)

	triageListCmd.Flags().IntP("limit", "l", 50, "Maximum number of responsibilities to fetch")
}
