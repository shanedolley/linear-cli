package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/shanedolley/lincli/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage time schedules (on-call rotations)",
	Long: `Manage time schedules: named on-call rotations of start/end windows per user.

Schedule entries are given as --entry "START|END|USER", where START and END are
ISO-8601 timestamps (e.g. 2026-07-10T09:00:00Z) and USER is an email, a user ID,
or a name reference. Repeat --entry for each window.

Schedules resolve by name or ID for get/update/delete.

Examples:
  lincli schedule list
  lincli schedule create --name "On-call" \
    --entry "2026-07-10T09:00:00Z|2026-07-17T09:00:00Z|alice@example.com"
  lincli schedule get "On-call"
  lincli schedule refresh <id>`,
}

var scheduleListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List time schedules",
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := scheduleClient()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		newerThan, _ := cmd.Flags().GetString("newer-than")
		createdAtISO, err := utils.ParseTimeExpression(newerThan)
		if err != nil {
			return fmt.Errorf("Invalid newer-than value: %v", err)
		}

		resp, err := api.ListTimeSchedules(ctx, client, limitPtr, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list schedules: %v", err)
		}

		if resp.TimeSchedules == nil || len(resp.TimeSchedules.Nodes) == 0 {
			output.Info("No time schedules found", plaintext, jsonOut)
			return nil
		}

		// timeSchedules has no server-side filter, so --newer-than is applied
		// client-side against each schedule's createdAt. Filtering in place
		// reuses the slice's own element type, avoiding the verbose generated
		// node type name. An empty threshold (the all_time default) keeps all.
		nodes := resp.TimeSchedules.Nodes[:0]
		for _, node := range resp.TimeSchedules.Nodes {
			if scheduleNewerThan(node.TimeScheduleFields.CreatedAt, createdAtISO) {
				nodes = append(nodes, node)
			}
		}
		if len(nodes) == 0 {
			output.Info("No time schedules found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(nodes)
			return nil
		}

		headers := []string{"ID", "Name", "Entries", "Integration", "External ID"}
		rows := make([][]string, len(nodes))
		for i, node := range nodes {
			f := node.TimeScheduleFields
			integration := "-"
			if f.Integration != nil {
				integration = f.Integration.Service
			}
			externalID := "-"
			if f.ExternalId != nil && *f.ExternalId != "" {
				externalID = *f.ExternalId
			}
			rows[i] = []string{
				f.Id,
				truncateString(f.Name, 30),
				fmt.Sprintf("%d", len(f.Entries)),
				integration,
				truncateString(externalID, 20),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d schedules\n", color.New(color.FgGreen).Sprint("✓"), len(nodes))
		}
		return nil
	},
}

var scheduleGetCmd = &cobra.Command{
	Use:     "get <name-or-id>",
	Aliases: []string{"show"},
	Short:   "Get a time schedule by name or ID",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut := viper.GetBool("json")

		client, ctx, err := scheduleClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		id, err := resolveTimeSchedule(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		resp, err := api.GetTimeSchedule(ctx, client, id)
		if err != nil {
			return fmt.Errorf("Failed to get schedule: %v", err)
		}
		f := resp.TimeSchedule.TimeScheduleFields

		if jsonOut {
			output.JSON(resp.TimeSchedule)
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Schedule:"), f.Name)
		fmt.Printf("  ID:      %s\n", f.Id)
		if f.Integration != nil {
			fmt.Printf("  Integration: %s\n", f.Integration.Service)
		}
		if f.ExternalId != nil && *f.ExternalId != "" {
			fmt.Printf("  External ID:  %s\n", *f.ExternalId)
		}
		fmt.Printf("  Entries: %d\n", len(f.Entries))
		for _, e := range f.Entries {
			who := "-"
			if e.UserEmail != nil && *e.UserEmail != "" {
				who = *e.UserEmail
			} else if e.UserId != nil && *e.UserId != "" {
				who = *e.UserId
			}
			fmt.Printf("    %s → %s  %s\n", e.StartsAt.Format(time.RFC3339), e.EndsAt.Format(time.RFC3339), who)
		}
		return nil
	},
}

var scheduleCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a time schedule",
	Long: `Create a time schedule. --name and at least one --entry are required.
Each --entry is "START|END|USER" (ISO-8601 timestamps; USER is an email, ID, or name).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return errors.New("Name is required (--name)")
		}

		entryStrs, _ := cmd.Flags().GetStringArray("entry")
		if len(entryStrs) == 0 {
			return errors.New("At least one --entry is required")
		}
		entries, err := parseScheduleEntries(entryStrs)
		if err != nil {
			return err
		}

		client, ctx, err := scheduleClient()
		if err != nil {
			return err
		}

		input := &api.TimeScheduleCreateInput{Name: name, Entries: entries}
		if cmd.Flags().Changed("external-id") {
			externalID, _ := cmd.Flags().GetString("external-id")
			input.ExternalId = &externalID
		}
		if cmd.Flags().Changed("external-url") {
			externalURL, _ := cmd.Flags().GetString("external-url")
			input.ExternalUrl = &externalURL
		}

		resp, err := api.TimeScheduleCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to create schedule: %v", err)
		}
		if resp.TimeScheduleCreate == nil || !resp.TimeScheduleCreate.Success {
			return errors.New("Failed to create schedule")
		}

		if jsonOut {
			output.JSON(resp.TimeScheduleCreate.TimeSchedule)
		} else {
			output.Success(fmt.Sprintf("Created schedule %s", resp.TimeScheduleCreate.TimeSchedule.TimeScheduleFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var scheduleUpdateCmd = &cobra.Command{
	Use:   "update <name-or-id>",
	Short: "Update a time schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := scheduleClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		id, err := resolveTimeSchedule(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		input := &api.TimeScheduleUpdateInput{}
		changed := false

		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
			changed = true
		}
		if cmd.Flags().Changed("entry") {
			entryStrs, _ := cmd.Flags().GetStringArray("entry")
			entries, err := parseScheduleEntries(entryStrs)
			if err != nil {
				return err
			}
			input.Entries = entries
			changed = true
		}
		if cmd.Flags().Changed("external-id") {
			externalID, _ := cmd.Flags().GetString("external-id")
			input.ExternalId = &externalID
			changed = true
		}
		if cmd.Flags().Changed("external-url") {
			externalURL, _ := cmd.Flags().GetString("external-url")
			input.ExternalUrl = &externalURL
			changed = true
		}

		if !changed {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.TimeScheduleUpdate(ctx, client, id, input)
		if err != nil {
			return fmt.Errorf("Failed to update schedule: %v", err)
		}
		if resp.TimeScheduleUpdate == nil || !resp.TimeScheduleUpdate.Success {
			return errors.New("Failed to update schedule")
		}

		if jsonOut {
			output.JSON(resp.TimeScheduleUpdate.TimeSchedule)
		} else {
			output.Success(fmt.Sprintf("Updated schedule %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var scheduleDeleteCmd = &cobra.Command{
	Use:     "delete <name-or-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a time schedule",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := scheduleClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		id, err := resolveTimeSchedule(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		resp, err := api.TimeScheduleDelete(ctx, client, id)
		if err != nil {
			return fmt.Errorf("Failed to delete schedule: %v", err)
		}
		if resp.TimeScheduleDelete == nil || !resp.TimeScheduleDelete.Success {
			return errors.New("Failed to delete schedule")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": id})
		} else {
			output.Success(fmt.Sprintf("Deleted schedule %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var scheduleUpsertExternalCmd = &cobra.Command{
	Use:   "upsert-external",
	Short: "Create or update a schedule keyed on an external ID",
	Long: `Create or update a time schedule, keyed on --external-id. If a schedule with
that external identifier exists it is updated, otherwise a new one is created.
--external-id is required; pass --name and/or --entry to set its contents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		externalID, _ := cmd.Flags().GetString("external-id")
		if externalID == "" {
			return errors.New("External ID is required (--external-id)")
		}

		client, ctx, err := scheduleClient()
		if err != nil {
			return err
		}

		input := &api.TimeScheduleUpdateInput{}
		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
		}
		if cmd.Flags().Changed("entry") {
			entryStrs, _ := cmd.Flags().GetStringArray("entry")
			entries, err := parseScheduleEntries(entryStrs)
			if err != nil {
				return err
			}
			input.Entries = entries
		}
		if cmd.Flags().Changed("external-url") {
			externalURL, _ := cmd.Flags().GetString("external-url")
			input.ExternalUrl = &externalURL
		}

		resp, err := api.TimeScheduleUpsertExternal(ctx, client, externalID, input)
		if err != nil {
			return fmt.Errorf("Failed to upsert schedule: %v", err)
		}
		if resp.TimeScheduleUpsertExternal == nil || !resp.TimeScheduleUpsertExternal.Success {
			return errors.New("Failed to upsert schedule")
		}

		if jsonOut {
			output.JSON(resp.TimeScheduleUpsertExternal.TimeSchedule)
		} else {
			output.Success(fmt.Sprintf("Upserted schedule %s", resp.TimeScheduleUpsertExternal.TimeSchedule.TimeScheduleFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var scheduleRefreshCmd = &cobra.Command{
	Use:   "refresh <name-or-id>",
	Short: "Refresh an integration-backed schedule",
	Long:  `Refresh a schedule's entries from its backing integration.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := scheduleClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		id, err := resolveTimeSchedule(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		resp, err := api.TimeScheduleRefreshIntegrationSchedule(ctx, client, id)
		if err != nil {
			return fmt.Errorf("Failed to refresh schedule: %v", err)
		}
		if resp.TimeScheduleRefreshIntegrationSchedule == nil || !resp.TimeScheduleRefreshIntegrationSchedule.Success {
			return errors.New("Failed to refresh schedule")
		}

		if jsonOut {
			output.JSON(resp.TimeScheduleRefreshIntegrationSchedule.TimeSchedule)
		} else {
			output.Success(fmt.Sprintf("Refreshed schedule %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// parseScheduleEntries parses each "START|END|USER" string into a schedule
// entry input, failing on the first malformed entry.
func parseScheduleEntries(entryStrs []string) ([]*api.TimeScheduleEntryInput, error) {
	entries := make([]*api.TimeScheduleEntryInput, 0, len(entryStrs))
	for _, s := range entryStrs {
		entry, err := parseScheduleEntry(s)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// parseScheduleEntry parses a single "START|END|USER" schedule entry. START and
// END are ISO-8601 (RFC 3339) timestamps. USER is mapped to a Linear user id
// when it is a UUID, otherwise to userEmail (which accepts an email or a name
// reference for external users that can't be mapped to a Linear id).
func parseScheduleEntry(s string) (*api.TimeScheduleEntryInput, error) {
	parts := strings.Split(s, "|")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid entry %q: expected START|END|USER", s)
	}
	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])
	user := strings.TrimSpace(parts[2])

	startsAt, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return nil, fmt.Errorf("invalid start time %q in entry: expected ISO-8601 (e.g. 2026-07-10T09:00:00Z)", start)
	}
	endsAt, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return nil, fmt.Errorf("invalid end time %q in entry: expected ISO-8601 (e.g. 2026-07-10T09:00:00Z)", end)
	}
	if user == "" {
		return nil, fmt.Errorf("invalid entry %q: a user (email, ID, or name) is required", s)
	}

	entry := &api.TimeScheduleEntryInput{StartsAt: startsAt, EndsAt: endsAt}
	if isUUID(user) {
		entry.UserId = &user
	} else {
		entry.UserEmail = &user
	}
	return entry, nil
}

// scheduleNewerThan reports whether a schedule created at createdAt passes the
// --newer-than threshold given as an ISO-8601 string. An empty threshold (the
// all_time default) or an unparseable one passes every schedule.
func scheduleNewerThan(createdAt time.Time, thresholdISO string) bool {
	if thresholdISO == "" {
		return true
	}
	threshold, err := time.Parse(time.RFC3339, thresholdISO)
	if err != nil {
		return true
	}
	return !createdAt.Before(threshold)
}

// scheduleClient builds the authenticated client and context shared by the
// schedule subcommands.
func scheduleClient() (graphql.Client, context.Context, error) {
	client, err := newGraphQLClient()
	if err != nil {
		return nil, nil, err
	}
	return client, context.Background(), nil
}

func init() {
	rootCmd.AddCommand(scheduleCmd)

	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleGetCmd)
	scheduleCmd.AddCommand(scheduleCreateCmd)
	scheduleCmd.AddCommand(scheduleUpdateCmd)
	scheduleCmd.AddCommand(scheduleDeleteCmd)
	scheduleCmd.AddCommand(scheduleUpsertExternalCmd)
	scheduleCmd.AddCommand(scheduleRefreshCmd)

	scheduleListCmd.Flags().IntP("limit", "l", 50, "Maximum number of schedules to fetch")
	scheduleListCmd.Flags().StringP("newer-than", "n", "all_time", "Show schedules created after this time (e.g. 3_months_ago; default all_time)")

	scheduleCreateCmd.Flags().String("name", "", "Schedule name (required)")
	scheduleCreateCmd.Flags().StringArray("entry", nil, "Schedule entry \"START|END|USER\" (repeatable; required)")
	scheduleCreateCmd.Flags().String("external-id", "", "External identifier for integration sync")
	scheduleCreateCmd.Flags().String("external-url", "", "URL to the external schedule")

	scheduleUpdateCmd.Flags().String("name", "", "New schedule name")
	scheduleUpdateCmd.Flags().StringArray("entry", nil, "Replacement schedule entry \"START|END|USER\" (repeatable)")
	scheduleUpdateCmd.Flags().String("external-id", "", "New external identifier")
	scheduleUpdateCmd.Flags().String("external-url", "", "New external URL")

	scheduleUpsertExternalCmd.Flags().String("external-id", "", "External identifier to key on (required)")
	scheduleUpsertExternalCmd.Flags().String("name", "", "Schedule name")
	scheduleUpsertExternalCmd.Flags().StringArray("entry", nil, "Schedule entry \"START|END|USER\" (repeatable)")
	scheduleUpsertExternalCmd.Flags().String("external-url", "", "URL to the external schedule")
}
