package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/shanedolley/lincli/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// parseDateTimeFlag parses a user-supplied date/time flag into a time.Time.
// It accepts a plain date (YYYY-MM-DD) or a full RFC3339 timestamp, matching
// the formats the rest of the CLI accepts for time input.
func parseDateTimeFlag(value string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid date '%s' (expected YYYY-MM-DD or RFC3339)", value)
}

var cycleCmd = &cobra.Command{
	Use:   "cycle",
	Short: "Manage Linear cycles",
	Long: `Manage Linear cycles: time-boxed iterations scoped to a team.

Cycle writes (create, update, archive, shift, start-now) require the team to
have cycles enabled. On a team where cycles are off, Linear rejects the write
and the error is shown as-is.

Examples:
  lincli cycle list --team ENG                 # List a team's cycles
  lincli cycle get CYCLE-ID                     # Get cycle details
  lincli cycle create --team ENG --starts-at 2026-08-01 --ends-at 2026-08-14
  lincli cycle update CYCLE-ID --name "Sprint 12"
  lincli cycle archive CYCLE-ID
  lincli cycle shift CYCLE-ID --by 7            # Shift this cycle onward by 7 days
  lincli cycle start-now CYCLE-ID               # Start the upcoming cycle today`,
}

var cycleListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List cycles",
	Long:    `List cycles, optionally filtered by team.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		sortBy, _ := cmd.Flags().GetString("sort")
		var orderByEnum *api.PaginationOrderBy
		switch sortBy {
		case "", "linear":
			orderByEnum = nil
		case "created", "createdAt":
			val := api.PaginationOrderByCreatedat
			orderByEnum = &val
		case "updated", "updatedAt":
			val := api.PaginationOrderByUpdatedat
			orderByEnum = &val
		default:
			return fmt.Errorf("Invalid sort option: %s. Valid options are: linear, created, updated", sortBy)
		}

		filter, err := buildCycleFilterTyped(ctx, client, cache, cmd)
		if err != nil {
			return err
		}

		resp, err := api.ListCycles(ctx, client, &filter, limitPtr, nil, orderByEnum)
		if err != nil {
			return fmt.Errorf("Failed to list cycles: %v", err)
		}

		if len(resp.Cycles.Nodes) == 0 {
			output.Info("No cycles found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.Cycles.Nodes)
			return nil
		}

		if plaintext {
			fmt.Println("# Cycles")
			for _, node := range resp.Cycles.Nodes {
				f := node.CycleListFields
				fmt.Printf("## %s\n", cycleLabel(f))
				fmt.Printf("- **ID**: %s\n", f.Id)
				fmt.Printf("- **Team**: %s\n", f.Team.Key)
				fmt.Printf("- **Starts**: %s\n", f.StartsAt.Format("2006-01-02"))
				fmt.Printf("- **Ends**: %s\n", f.EndsAt.Format("2006-01-02"))
				fmt.Printf("- **Progress**: %.0f%%\n", f.Progress*100)
				fmt.Println()
			}
			fmt.Printf("\nTotal: %d cycles\n", len(resp.Cycles.Nodes))
			return nil
		}

		headers := []string{"ID", "Number", "Name", "Team", "Starts", "Ends", "Progress"}
		rows := make([][]string, len(resp.Cycles.Nodes))
		for i, node := range resp.Cycles.Nodes {
			f := node.CycleListFields
			name := ""
			if f.Name != nil {
				name = *f.Name
			}
			rows[i] = []string{
				f.Id,
				fmt.Sprintf("%.0f", f.Number),
				truncateString(name, 24),
				f.Team.Key,
				f.StartsAt.Format("2006-01-02"),
				f.EndsAt.Format("2006-01-02"),
				fmt.Sprintf("%.0f%%", f.Progress*100),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d cycles\n", color.New(color.FgGreen).Sprint("✓"), len(resp.Cycles.Nodes))
		}
		return nil
	},
}

var cycleGetCmd = &cobra.Command{
	Use:     "get <cycle-id>",
	Aliases: []string{"show"},
	Short:   "Get cycle details",
	Long:    `Get detailed information about a cycle, including its scope and progress history.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		cycleID, err := resolveCycleArg(ctx, client, cache, cmd, args[0])
		if err != nil {
			return err
		}
		resp, err := api.GetCycle(ctx, client, cycleID)
		if err != nil {
			return fmt.Errorf("Failed to get cycle: %v", err)
		}
		if resp.Cycle == nil {
			return fmt.Errorf("Cycle not found: %s", args[0])
		}
		d := resp.Cycle.CycleDetailFields
		f := d.CycleListFields

		if jsonOut {
			output.JSON(resp.Cycle)
			return nil
		}

		label := cycleLabel(f)
		if plaintext {
			fmt.Printf("# %s\n\n", label)
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Team**: %s\n", f.Team.Key)
			if f.Description != nil && *f.Description != "" {
				fmt.Printf("- **Description**: %s\n", *f.Description)
			}
			fmt.Printf("- **Starts**: %s\n", f.StartsAt.Format("2006-01-02"))
			fmt.Printf("- **Ends**: %s\n", f.EndsAt.Format("2006-01-02"))
			fmt.Printf("- **Progress**: %.0f%%\n", f.Progress*100)
			if f.CompletedAt != nil {
				fmt.Printf("- **Completed**: %s\n", f.CompletedAt.Format("2006-01-02"))
			}
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Cycle:"), label)
		fmt.Printf("  ID:       %s\n", f.Id)
		fmt.Printf("  Team:     %s\n", f.Team.Key)
		fmt.Printf("  Starts:   %s\n", f.StartsAt.Format("2006-01-02"))
		fmt.Printf("  Ends:     %s\n", f.EndsAt.Format("2006-01-02"))
		fmt.Printf("  Progress: %.0f%%\n", f.Progress*100)
		if f.CompletedAt != nil {
			fmt.Printf("  Completed: %s\n", f.CompletedAt.Format("2006-01-02"))
		}
		return nil
	},
}

var cycleCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a new cycle",
	Long:    `Create a new cycle for a team. --starts-at and --ends-at are required.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		team, _ := cmd.Flags().GetString("team")
		if team == "" {
			return errors.New("Team is required (--team)")
		}
		startsAt, _ := cmd.Flags().GetString("starts-at")
		endsAt, _ := cmd.Flags().GetString("ends-at")
		if startsAt == "" || endsAt == "" {
			return errors.New("Both --starts-at and --ends-at are required")
		}

		start, err := parseDateTimeFlag(startsAt)
		if err != nil {
			return err
		}
		end, err := parseDateTimeFlag(endsAt)
		if err != nil {
			return err
		}

		teamID, err := resolveTeam(ctx, client, cache, team)
		if err != nil {
			return err
		}

		input := api.CycleCreateInput{
			TeamId:   teamID,
			StartsAt: start,
			EndsAt:   end,
		}
		if name, _ := cmd.Flags().GetString("name"); name != "" {
			input.Name = &name
		}
		if description, _ := cmd.Flags().GetString("description"); description != "" {
			input.Description = &description
		}

		resp, err := api.CycleCreate(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to create cycle: %v", err)
		}
		if !resp.CycleCreate.Success {
			return errors.New("Failed to create cycle")
		}

		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		if jsonOut {
			output.JSON(resp.CycleCreate.Cycle)
		} else if resp.CycleCreate.Cycle != nil {
			output.Success(fmt.Sprintf("Created cycle %s", cycleLabel(resp.CycleCreate.Cycle.CycleListFields)), plaintext, jsonOut)
		} else {
			output.Success("Created cycle", plaintext, jsonOut)
		}
		return nil
	},
}

var cycleUpdateCmd = &cobra.Command{
	Use:   "update <cycle-id>",
	Short: "Update a cycle",
	Long:  `Update a cycle's name, description, start date, or end date.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		cycleID, err := resolveCycleArg(ctx, client, cache, cmd, args[0])
		if err != nil {
			return err
		}

		input := api.CycleUpdateInput{}
		hasUpdates := false

		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
			hasUpdates = true
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
			hasUpdates = true
		}
		if cmd.Flags().Changed("starts-at") {
			startsAt, _ := cmd.Flags().GetString("starts-at")
			start, err := parseDateTimeFlag(startsAt)
			if err != nil {
				return err
			}
			input.StartsAt = &start
			hasUpdates = true
		}
		if cmd.Flags().Changed("ends-at") {
			endsAt, _ := cmd.Flags().GetString("ends-at")
			end, err := parseDateTimeFlag(endsAt)
			if err != nil {
				return err
			}
			input.EndsAt = &end
			hasUpdates = true
		}

		if !hasUpdates {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.CycleUpdate(ctx, client, cycleID, &input)
		if err != nil {
			return fmt.Errorf("Failed to update cycle: %v", err)
		}
		if !resp.CycleUpdate.Success {
			return errors.New("Failed to update cycle")
		}

		if jsonOut {
			output.JSON(resp.CycleUpdate.Cycle)
		} else {
			output.Success(fmt.Sprintf("Updated cycle %s", cycleID), plaintext, jsonOut)
		}
		return nil
	},
}

var cycleArchiveCmd = &cobra.Command{
	Use:   "archive <cycle-id>",
	Short: "Archive a cycle",
	Long:  `Archive a cycle. This action executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		cycleID, err := resolveCycleArg(ctx, client, cache, cmd, args[0])
		if err != nil {
			return err
		}

		resp, err := api.CycleArchive(ctx, client, cycleID)
		if err != nil {
			return fmt.Errorf("Failed to archive cycle: %v", err)
		}
		if !resp.CycleArchive.Success {
			return errors.New("Failed to archive cycle")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": cycleID})
		} else {
			output.Success(fmt.Sprintf("Archived cycle %s", cycleID), plaintext, jsonOut)
		}
		return nil
	},
}

var cycleShiftCmd = &cobra.Command{
	Use:   "shift <cycle-id>",
	Short: "Shift cycles by a number of days",
	Long: `Shift the given cycle and all cycles after it by a number of days.

Takes the id of the cycle to start shifting from. Use --by with a positive
number to move later, or a negative number to move earlier.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		days, _ := cmd.Flags().GetFloat64("by")
		if days == 0 {
			return errors.New("Specify a non-zero number of days with --by")
		}

		cycleID, err := resolveCycleArg(ctx, client, cache, cmd, args[0])
		if err != nil {
			return err
		}

		input := api.CycleShiftAllInput{Id: cycleID, DaysToShift: days}
		resp, err := api.CycleShiftAll(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to shift cycles: %v", err)
		}
		if !resp.CycleShiftAll.Success {
			return errors.New("Failed to shift cycles")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": cycleID, "daysShifted": days})
		} else {
			output.Success(fmt.Sprintf("Shifted cycles from %s by %.0f days", cycleID, days), plaintext, jsonOut)
		}
		return nil
	},
}

var cycleStartNowCmd = &cobra.Command{
	Use:   "start-now <cycle-id>",
	Short: "Start the upcoming cycle today",
	Long: `Start the upcoming (next, not-yet-started) cycle as of midnight today.

Takes the id of that upcoming cycle. Completes the previous cycle if it has
not yet ended.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		cycleID, err := resolveCycleArg(ctx, client, cache, cmd, args[0])
		if err != nil {
			return err
		}

		resp, err := api.CycleStartUpcomingCycleToday(ctx, client, cycleID)
		if err != nil {
			return fmt.Errorf("Failed to start cycle: %v", err)
		}
		if !resp.CycleStartUpcomingCycleToday.Success {
			return errors.New("Failed to start cycle")
		}

		if jsonOut {
			output.JSON(resp.CycleStartUpcomingCycleToday.Cycle)
		} else {
			output.Success(fmt.Sprintf("Started cycle %s today", cycleID), plaintext, jsonOut)
		}
		return nil
	},
}

// buildCycleFilterTyped builds a CycleFilter from the list flags: an optional
// team filter (by resolved team ID) and a created-after bound from
// --newer-than (which defaults to all_time for cycles).
func buildCycleFilterTyped(ctx context.Context, client graphql.Client, cache *ResolverCache, cmd *cobra.Command) (api.CycleFilter, error) {
	filter := api.CycleFilter{}

	if team, _ := cmd.Flags().GetString("team"); team != "" {
		teamID, err := resolveTeam(ctx, client, cache, team)
		if err != nil {
			return filter, err
		}
		filter.Team = &api.TeamFilter{Id: &api.IDComparator{Eq: &teamID}}
	}

	newerThan, _ := cmd.Flags().GetString("newer-than")
	createdAt, err := utils.ParseTimeExpression(newerThan)
	if err != nil {
		return filter, fmt.Errorf("Invalid newer-than value: %v", err)
	}
	if createdAt != "" {
		filter.CreatedAt = &api.DateComparator{Gte: &createdAt}
	}

	return filter, nil
}

// resolveCycleArg resolves a cycle argument that may be a UUID (returned as-is)
// or a cycle number/name. Number/name resolution needs a team, taken from the
// --team flag; without it, a non-UUID reference is an error.
func resolveCycleArg(ctx context.Context, client graphql.Client, cache *ResolverCache, cmd *cobra.Command, ref string) (string, error) {
	if isUUID(ref) {
		return ref, nil
	}
	team, _ := cmd.Flags().GetString("team")
	if team == "" {
		return "", fmt.Errorf("Resolving cycle '%s' by number or name requires --team (or pass a cycle UUID)", ref)
	}
	teamID, err := resolveTeam(ctx, client, cache, team)
	if err != nil {
		return "", err
	}
	cycleID, err := resolveCycle(ctx, client, cache, teamID, ref)
	if err != nil {
		return "", err
	}
	return cycleID, nil
}

// cycleLabel builds a human-friendly label for a cycle: its name if set,
// otherwise "Cycle <number>".
func cycleLabel(f api.CycleListFields) string {
	if f.Name != nil && *f.Name != "" {
		return *f.Name
	}
	return fmt.Sprintf("Cycle %.0f", f.Number)
}

func init() {
	rootCmd.AddCommand(cycleCmd)
	cycleCmd.AddCommand(cycleListCmd)
	cycleCmd.AddCommand(cycleGetCmd)
	cycleCmd.AddCommand(cycleCreateCmd)
	cycleCmd.AddCommand(cycleUpdateCmd)
	cycleCmd.AddCommand(cycleArchiveCmd)
	cycleCmd.AddCommand(cycleShiftCmd)
	cycleCmd.AddCommand(cycleStartNowCmd)

	cycleListCmd.Flags().StringP("team", "t", "", "Filter by team key")
	cycleListCmd.Flags().IntP("limit", "l", 50, "Maximum number of cycles to fetch")
	cycleListCmd.Flags().StringP("sort", "o", "linear", "Sort order: linear (default), created, updated")
	cycleListCmd.Flags().StringP("newer-than", "n", "all_time", "Show cycles created after this time (e.g. 3_months_ago; default all_time)")

	// get/update/archive/shift/start-now take a cycle UUID directly, or a
	// number/name plus --team to resolve within that team.
	cycleGetCmd.Flags().StringP("team", "t", "", "Team key (needed to resolve a cycle by number or name)")

	cycleCreateCmd.Flags().StringP("team", "t", "", "Team key (required)")
	cycleCreateCmd.Flags().String("name", "", "Cycle name")
	cycleCreateCmd.Flags().StringP("description", "d", "", "Cycle description")
	cycleCreateCmd.Flags().String("starts-at", "", "Start date (YYYY-MM-DD, required)")
	cycleCreateCmd.Flags().String("ends-at", "", "End date (YYYY-MM-DD, required)")

	cycleUpdateCmd.Flags().StringP("team", "t", "", "Team key (needed to resolve a cycle by number or name)")
	cycleUpdateCmd.Flags().String("name", "", "Cycle name")
	cycleUpdateCmd.Flags().StringP("description", "d", "", "Cycle description")
	cycleUpdateCmd.Flags().String("starts-at", "", "Start date (YYYY-MM-DD)")
	cycleUpdateCmd.Flags().String("ends-at", "", "End date (YYYY-MM-DD)")

	cycleArchiveCmd.Flags().StringP("team", "t", "", "Team key (needed to resolve a cycle by number or name)")

	cycleShiftCmd.Flags().StringP("team", "t", "", "Team key (needed to resolve a cycle by number or name)")
	cycleShiftCmd.Flags().Float64("by", 0, "Number of days to shift (negative to move earlier)")

	cycleStartNowCmd.Flags().StringP("team", "t", "", "Team key (needed to resolve a cycle by number or name)")
}
