package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/shanedolley/lincli/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// initiativeStatusValues lists the real InitiativeStatus enum values (verified
// against schema.graphql), used both for --status validation and to build the
// "valid values" error message.
var initiativeStatusValues = []string{
	string(api.InitiativeStatusProposed),
	string(api.InitiativeStatusPlanned),
	string(api.InitiativeStatusActive),
	string(api.InitiativeStatusCompleted),
	string(api.InitiativeStatusCanceled),
}

// parseInitiativeStatus validates a --status value against InitiativeStatus
// (case-insensitively) and returns the canonical enum value.
func parseInitiativeStatus(status string) (api.InitiativeStatus, error) {
	for _, v := range initiativeStatusValues {
		if strings.EqualFold(v, status) {
			return api.InitiativeStatus(v), nil
		}
	}
	return "", fmt.Errorf("Invalid status '%s'. Valid values: %s", status, strings.Join(initiativeStatusValues, ", "))
}

// initiativeUpdateHealthValues lists the real InitiativeUpdateHealthType enum
// values (verified against schema.graphql), used both for --health
// validation on initiative status-update posts and to build the "valid
// values" error message.
var initiativeUpdateHealthValues = []string{
	string(api.InitiativeUpdateHealthTypeOntrack),
	string(api.InitiativeUpdateHealthTypeAtrisk),
	string(api.InitiativeUpdateHealthTypeOfftrack),
}

// parseInitiativeUpdateHealth validates a --health value against
// InitiativeUpdateHealthType (case-insensitively) and returns the canonical
// enum value.
func parseInitiativeUpdateHealth(health string) (api.InitiativeUpdateHealthType, error) {
	for _, v := range initiativeUpdateHealthValues {
		if strings.EqualFold(v, health) {
			return api.InitiativeUpdateHealthType(v), nil
		}
	}
	return "", fmt.Errorf("Invalid health '%s'. Valid values: %s", health, strings.Join(initiativeUpdateHealthValues, ", "))
}

// initiativeStatusColor returns the display color for an initiative status,
// mirroring the state-coloring convention used for projects and issues.
func initiativeStatusColor(status api.InitiativeStatus) *color.Color {
	switch status {
	case api.InitiativeStatusPlanned:
		return color.New(color.FgCyan)
	case api.InitiativeStatusActive:
		return color.New(color.FgBlue)
	case api.InitiativeStatusCompleted:
		return color.New(color.FgGreen)
	case api.InitiativeStatusCanceled:
		return color.New(color.FgRed)
	default: // Proposed
		return color.New(color.FgWhite)
	}
}

// initiativeCmd represents the initiative command
var initiativeCmd = &cobra.Command{
	Use:   "initiative",
	Short: "Manage Linear initiatives",
	Long: `Manage Linear initiatives: high-level strategic groupings of projects
toward a business goal.

Examples:
  lincli initiative list                            # List all initiatives
  lincli initiative list --include-archived         # Include archived initiatives
  lincli initiative get "Platform Modernization"    # Get initiative details
  lincli initiative create --name "Q3 Growth" --status Planned
  lincli initiative update ID --status Active
  lincli initiative archive ID
  lincli initiative set-lead ID --team ENG
  lincli initiative project add ID lincli-sandbox
  lincli initiative label add ID "Strategic"
  lincli initiative relate PARENT-ID CHILD-ID
  lincli initiative update-post create ID --body "Kicking off" --health onTrack
  lincli initiative update-reminder ID`,
}

var initiativeListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List initiatives",
	Long:    `List all initiatives in your Linear workspace.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		sortBy, _ := cmd.Flags().GetString("sort")
		var orderByEnum *api.PaginationOrderBy
		if sortBy != "" {
			switch sortBy {
			case "created", "createdAt":
				val := api.PaginationOrderByCreatedat
				orderByEnum = &val
			case "updated", "updatedAt":
				val := api.PaginationOrderByUpdatedat
				orderByEnum = &val
			case "linear":
				// Use nil for Linear's default sort
				orderByEnum = nil
			default:
				return fmt.Errorf("Invalid sort option: %s. Valid options are: linear, created, updated", sortBy)
			}
		}

		filter, err := buildInitiativeFilterTyped(cmd)
		if err != nil {
			return err
		}

		includeArchived, _ := cmd.Flags().GetBool("include-archived")
		includeArchivedPtr := &includeArchived

		resp, err := api.ListInitiatives(ctx, client, &filter, limitPtr, nil, orderByEnum, includeArchivedPtr)
		if err != nil {
			return fmt.Errorf("Failed to list initiatives: %v", err)
		}

		if len(resp.Initiatives.Nodes) == 0 {
			output.Info("No initiatives found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.Initiatives.Nodes)
			return nil
		}

		if plaintext {
			fmt.Println("# Initiatives")
			for _, node := range resp.Initiatives.Nodes {
				f := node.InitiativeListFields
				fmt.Printf("## %s\n", f.Name)
				fmt.Printf("- **ID**: %s\n", f.Id)
				fmt.Printf("- **Status**: %s\n", f.Status)
				if f.Owner != nil {
					fmt.Printf("- **Owner**: %s\n", f.Owner.Name)
				} else {
					fmt.Printf("- **Owner**: Unassigned\n")
				}
				if f.TargetDate != nil {
					fmt.Printf("- **Target Date**: %s\n", *f.TargetDate)
				}
				if f.Health != nil {
					fmt.Printf("- **Health**: %s\n", *f.Health)
				}
				fmt.Printf("- **URL**: %s\n", f.Url)
				fmt.Println()
			}
			fmt.Printf("\nTotal: %d initiatives\n", len(resp.Initiatives.Nodes))
			return nil
		}

		// Table output.
		//
		// Note: Initiative has no `progress` field in the schema (unlike
		// Project), so `health` (onTrack/atRisk/offTrack) is shown in its
		// place as the closest available progress signal.
		headers := []string{"ID", "Name", "Status", "Owner", "Target Date", "Health"}
		rows := make([][]string, len(resp.Initiatives.Nodes))

		for i, node := range resp.Initiatives.Nodes {
			f := node.InitiativeListFields

			owner := color.New(color.FgYellow).Sprint("Unassigned")
			if f.Owner != nil {
				owner = f.Owner.Name
			}

			targetDate := ""
			if f.TargetDate != nil {
				targetDate = *f.TargetDate
			}

			health := ""
			if f.Health != nil {
				health = string(*f.Health)
			}

			statusColor := initiativeStatusColor(f.Status)

			rows[i] = []string{
				f.Id,
				truncateString(f.Name, 30),
				statusColor.Sprint(string(f.Status)),
				owner,
				targetDate,
				health,
			}
		}

		output.Table(output.TableData{
			Headers: headers,
			Rows:    rows,
		}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d initiatives\n",
				color.New(color.FgGreen).Sprint("✓"),
				len(resp.Initiatives.Nodes))
		}
		return nil
	},
}

var initiativeGetCmd = &cobra.Command{
	Use:     "get <initiative-id-or-name>",
	Aliases: []string{"show"},
	Short:   "Get initiative details",
	Long:    `Get detailed information about a specific initiative. Accepts an initiative ID or an exact (case-insensitive) name.`,
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

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		resp, err := api.GetInitiative(ctx, client, initiativeID)
		if err != nil {
			return fmt.Errorf("Failed to get initiative: %v", err)
		}
		if resp.Initiative == nil {
			return fmt.Errorf("initiative %q not found", initiativeID)
		}
		f := resp.Initiative.InitiativeDetailFields

		// Labels are fetched as a separate, best-effort request rather than
		// being part of InitiativeDetailFields. Initiative labels are a
		// workspace-gated feature, and querying `labels` inline with the
		// other detail fields fails the *entire* GetInitiative request when
		// the feature is off for the workspace (confirmed live) - the same
		// failure mode already documented for `identifier`. An error here is
		// treated as "no labels to show", not a hard failure.
		var labels []*api.GetInitiativeLabelsInitiativeLabelsInitiativeLabelConnectionNodesInitiativeLabel
		if labelsResp, labelsErr := api.GetInitiativeLabels(ctx, client, initiativeID); labelsErr == nil && labelsResp.Initiative != nil && labelsResp.Initiative.Labels != nil {
			labels = labelsResp.Initiative.Labels.Nodes
		}

		if jsonOut {
			output.JSON(struct {
				api.InitiativeDetailFields
				Labels []*api.GetInitiativeLabelsInitiativeLabelsInitiativeLabelConnectionNodesInitiativeLabel `json:"labels,omitempty"`
			}{InitiativeDetailFields: f, Labels: labels})
			return nil
		}

		if plaintext {
			fmt.Printf("# %s\n\n", f.Name)

			if f.Description != nil && *f.Description != "" {
				fmt.Printf("## Description\n%s\n\n", *f.Description)
			}
			if f.Content != nil && *f.Content != "" {
				fmt.Printf("## Content\n%s\n\n", *f.Content)
			}

			fmt.Printf("## Core Details\n")
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Slug ID**: %s\n", f.SlugId)
			fmt.Printf("- **Status**: %s\n", f.Status)
			if f.Health != nil {
				fmt.Printf("- **Health**: %s\n", *f.Health)
			}
			fmt.Printf("- **Priority**: %s\n", priorityToString(f.Priority))
			fmt.Printf("- **Visibility**: %s\n", f.Visibility)
			if f.Icon != nil && *f.Icon != "" {
				fmt.Printf("- **Icon**: %s\n", *f.Icon)
			}
			if f.Color != nil && *f.Color != "" {
				fmt.Printf("- **Color**: %s\n", *f.Color)
			}

			fmt.Printf("\n## Timeline\n")
			if f.StartedAt != nil {
				fmt.Printf("- **Started**: %s\n", f.StartedAt.Format("2006-01-02"))
			}
			if f.TargetDate != nil {
				fmt.Printf("- **Target Date**: %s\n", *f.TargetDate)
			}
			fmt.Printf("- **Created**: %s\n", f.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("- **Updated**: %s\n", f.UpdatedAt.Format("2006-01-02 15:04:05"))
			if f.CompletedAt != nil {
				fmt.Printf("- **Completed**: %s\n", f.CompletedAt.Format("2006-01-02 15:04:05"))
			}
			if f.CanceledAt != nil {
				fmt.Printf("- **Canceled**: %s\n", f.CanceledAt.Format("2006-01-02 15:04:05"))
			}
			if f.ArchivedAt != nil {
				fmt.Printf("- **Archived**: %s\n", f.ArchivedAt.Format("2006-01-02 15:04:05"))
			}

			fmt.Printf("\n## People\n")
			if f.Owner != nil {
				fmt.Printf("- **Owner**: %s (%s)\n", f.Owner.Name, f.Owner.Email)
			} else {
				fmt.Printf("- **Owner**: Unassigned\n")
			}
			if f.Creator != nil {
				fmt.Printf("- **Creator**: %s (%s)\n", f.Creator.Name, f.Creator.Email)
			}

			// Lead team. Status-update posts are a separate `update-post`
			// subcommand group, not shown here.
			if f.LeadTeam != nil {
				fmt.Printf("\n## Lead Team\n")
				fmt.Printf("- **%s** (%s)\n", f.LeadTeam.Name, f.LeadTeam.Key)
			}

			if f.Projects != nil && len(f.Projects.Nodes) > 0 {
				fmt.Printf("\n## Projects\n")
				for _, p := range f.Projects.Nodes {
					fmt.Printf("- %s (%s)\n", p.Name, p.Id)
				}
			}

			if len(labels) > 0 {
				fmt.Printf("\n## Labels\n")
				for _, l := range labels {
					fmt.Printf("- %s\n", l.Name)
				}
			}

			hasParents := f.ParentInitiatives != nil && len(f.ParentInitiatives.Nodes) > 0
			hasSubs := f.SubInitiatives != nil && len(f.SubInitiatives.Nodes) > 0
			if hasParents || hasSubs {
				fmt.Printf("\n## Relations\n")
				if hasParents {
					for _, p := range f.ParentInitiatives.Nodes {
						fmt.Printf("- Parent: %s (%s)\n", p.Name, p.Id)
					}
				}
				if hasSubs {
					for _, s := range f.SubInitiatives.Nodes {
						fmt.Printf("- Sub-initiative: %s (%s)\n", s.Name, s.Id)
					}
				}
			}

			fmt.Printf("\n## URL\n- %s\n", f.Url)
			return nil
		}

		// Rich display
		fmt.Println()
		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("🎯 Initiative:"), f.Name)
		fmt.Println(strings.Repeat("─", 50))

		fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("ID:"), f.Id)

		if f.Description != nil && *f.Description != "" {
			fmt.Printf("\n%s\n%s\n", color.New(color.Bold).Sprint("Description:"), *f.Description)
		}

		statusColor := initiativeStatusColor(f.Status)
		fmt.Printf("\n%s %s\n", color.New(color.Bold).Sprint("Status:"), statusColor.Sprint(string(f.Status)))

		if f.Health != nil {
			fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Health:"), string(*f.Health))
		}

		fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Priority:"), priorityToString(f.Priority))

		if f.StartedAt != nil || f.TargetDate != nil {
			fmt.Println()
			if f.StartedAt != nil {
				fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Started:"), f.StartedAt.Format("2006-01-02"))
			}
			if f.TargetDate != nil {
				fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Target Date:"), *f.TargetDate)
			}
		}

		if f.Owner != nil {
			fmt.Printf("\n%s %s (%s)\n",
				color.New(color.Bold).Sprint("Owner:"),
				f.Owner.Name,
				color.New(color.FgCyan).Sprint(f.Owner.Email))
		} else {
			fmt.Printf("\n%s %s\n", color.New(color.Bold).Sprint("Owner:"), color.New(color.FgRed).Sprint("Unassigned"))
		}

		if f.LeadTeam != nil {
			fmt.Printf("%s %s (%s)\n",
				color.New(color.Bold).Sprint("Lead Team:"),
				f.LeadTeam.Name,
				color.New(color.FgCyan).Sprint(f.LeadTeam.Key))
		}

		fmt.Printf("\n%s\n", color.New(color.Bold).Sprint("Timeline:"))
		fmt.Printf("  Created: %s\n", f.CreatedAt.Format("2006-01-02"))
		fmt.Printf("  Updated: %s\n", f.UpdatedAt.Format("2006-01-02"))
		if f.CompletedAt != nil {
			fmt.Printf("  Completed: %s\n", f.CompletedAt.Format("2006-01-02"))
		}
		if f.CanceledAt != nil {
			fmt.Printf("  Canceled: %s\n", f.CanceledAt.Format("2006-01-02"))
		}
		if f.ArchivedAt != nil {
			fmt.Printf("  Archived: %s\n", f.ArchivedAt.Format("2006-01-02"))
		}

		if f.Projects != nil && len(f.Projects.Nodes) > 0 {
			fmt.Printf("\n%s (%d)\n", color.New(color.Bold).Sprint("Projects:"), len(f.Projects.Nodes))
			for _, p := range f.Projects.Nodes {
				fmt.Printf("  - %s\n", p.Name)
			}
		}

		if len(labels) > 0 {
			fmt.Printf("\n%s\n", color.New(color.Bold).Sprint("Labels:"))
			for _, l := range labels {
				fmt.Printf("  - %s\n", l.Name)
			}
		}

		hasParents := f.ParentInitiatives != nil && len(f.ParentInitiatives.Nodes) > 0
		hasSubs := f.SubInitiatives != nil && len(f.SubInitiatives.Nodes) > 0
		if hasParents || hasSubs {
			fmt.Printf("\n%s\n", color.New(color.Bold).Sprint("Relations:"))
			if hasParents {
				for _, p := range f.ParentInitiatives.Nodes {
					fmt.Printf("  - Parent: %s\n", p.Name)
				}
			}
			if hasSubs {
				for _, s := range f.SubInitiatives.Nodes {
					fmt.Printf("  - Sub-initiative: %s\n", s.Name)
				}
			}
		}

		if f.Url != "" {
			fmt.Printf("\n%s %s\n",
				color.New(color.Bold).Sprint("URL:"),
				color.New(color.FgBlue, color.Underline).Sprint(f.Url))
		}

		fmt.Println()
		return nil
	},
}

var initiativeCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a new initiative",
	Long:    `Create a new initiative in Linear.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return errors.New("Name is required (--name)")
		}

		input := api.InitiativeCreateInput{Name: name}

		if description, _ := cmd.Flags().GetString("description"); description != "" {
			input.Description = &description
		}

		if owner, _ := cmd.Flags().GetString("owner"); owner != "" {
			ownerID, err := resolveUser(ctx, client, cache, owner)
			if err != nil {
				return fmt.Errorf("Failed to find owner '%s': %v", owner, err)
			}
			input.OwnerId = &ownerID
		}

		if targetDate, _ := cmd.Flags().GetString("target-date"); targetDate != "" {
			input.TargetDate = &targetDate
		}

		if status, _ := cmd.Flags().GetString("status"); status != "" {
			statusEnum, err := parseInitiativeStatus(status)
			if err != nil {
				return err
			}
			input.Status = &statusEnum
		}

		createResp, err := api.CreateInitiative(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to create initiative: %v", err)
		}

		if !createResp.InitiativeCreate.Success {
			return errors.New("Failed to create initiative")
		}

		initiative := createResp.InitiativeCreate.Initiative

		if jsonOut {
			output.JSON(initiative)
		} else if plaintext {
			fmt.Printf("Created initiative: %s\n", initiative.InitiativeListFields.Name)
		} else {
			fmt.Printf("%s Created initiative: %s\n",
				color.New(color.FgGreen).Sprint("✓"),
				color.New(color.FgCyan, color.Bold).Sprint(initiative.InitiativeListFields.Name))
			if initiative.InitiativeListFields.Owner != nil {
				fmt.Printf("  Owner: %s\n", color.New(color.FgCyan).Sprint(initiative.InitiativeListFields.Owner.Name))
			}
		}
		return nil
	},
}

var initiativeUpdateCmd = &cobra.Command{
	Use:   "update <initiative-id-or-name>",
	Short: "Update an initiative",
	Long: `Update an initiative's record: name, description, owner, target date, or status.

This edits the initiative itself; it does not post an initiative status update.

Examples:
  lincli initiative update ID --status Active
  lincli initiative update "Q3 Growth" --owner jane@example.com
  lincli initiative update ID --target-date 2025-12-31
  lincli initiative update ID --owner unassigned`,
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

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		input := api.InitiativeUpdateInput{}

		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
		}

		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
		}

		if cmd.Flags().Changed("owner") {
			owner, _ := cmd.Flags().GetString("owner")
			switch strings.ToLower(owner) {
			case "unassigned", "":
				// Set to nil to unassign, matching the assignee-clear
				// convention used by 'issue update --assignee unassigned'.
				var nilID *string
				input.OwnerId = nilID
			default:
				ownerID, err := resolveUser(ctx, client, cache, owner)
				if err != nil {
					return fmt.Errorf("Failed to find owner '%s': %v", owner, err)
				}
				input.OwnerId = &ownerID
			}
		}

		if cmd.Flags().Changed("target-date") {
			targetDate, _ := cmd.Flags().GetString("target-date")
			if targetDate == "" {
				var nilDate *string
				input.TargetDate = nilDate
			} else {
				input.TargetDate = &targetDate
			}
		}

		if cmd.Flags().Changed("status") {
			status, _ := cmd.Flags().GetString("status")
			statusEnum, err := parseInitiativeStatus(status)
			if err != nil {
				return err
			}
			input.Status = &statusEnum
		}

		hasUpdates := input.Name != nil ||
			input.Description != nil ||
			cmd.Flags().Changed("owner") ||
			cmd.Flags().Changed("target-date") ||
			input.Status != nil

		if !hasUpdates {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		updateResp, err := api.UpdateInitiative(ctx, client, initiativeID, &input)
		if err != nil {
			return fmt.Errorf("Failed to update initiative: %v", err)
		}

		if !updateResp.InitiativeUpdate.Success {
			return errors.New("Failed to update initiative")
		}

		updated := updateResp.InitiativeUpdate.Initiative

		if jsonOut {
			output.JSON(updated)
		} else if plaintext {
			fmt.Printf("Updated initiative %s\n", updated.InitiativeListFields.Name)
		} else {
			output.Success(fmt.Sprintf("Updated initiative %s", updated.InitiativeListFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeArchiveCmd = &cobra.Command{
	Use:   "archive <initiative-id-or-name>",
	Short: "Archive an initiative",
	Long:  `Archive an initiative. This action executes immediately with no confirmation prompt.`,
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

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		resp, err := api.InitiativeArchive(ctx, client, initiativeID)
		if err != nil {
			return fmt.Errorf("Failed to archive initiative: %v", err)
		}

		if !resp.InitiativeArchive.Success {
			return errors.New("Failed to archive initiative")
		}

		name := initiativeID
		if resp.InitiativeArchive.Entity != nil {
			name = resp.InitiativeArchive.Entity.Name
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": initiativeID})
		} else if plaintext {
			fmt.Printf("Archived initiative %s\n", name)
		} else {
			output.Success(fmt.Sprintf("Archived initiative %s", name), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <initiative-id-or-name>",
	Short: "Unarchive an initiative",
	Long:  `Unarchive a previously archived initiative. This action executes immediately with no confirmation prompt.`,
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

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		resp, err := api.InitiativeUnarchive(ctx, client, initiativeID)
		if err != nil {
			return fmt.Errorf("Failed to unarchive initiative: %v", err)
		}

		if !resp.InitiativeUnarchive.Success {
			return errors.New("Failed to unarchive initiative")
		}

		name := initiativeID
		if resp.InitiativeUnarchive.Entity != nil {
			name = resp.InitiativeUnarchive.Entity.Name
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": initiativeID})
		} else if plaintext {
			fmt.Printf("Unarchived initiative %s\n", name)
		} else {
			output.Success(fmt.Sprintf("Unarchived initiative %s", name), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeDeleteCmd = &cobra.Command{
	Use:   "delete <initiative-id-or-name>",
	Short: "Delete an initiative",
	Long:  `Delete (trash) an initiative. This action executes immediately with no confirmation prompt.`,
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

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		resp, err := api.InitiativeDelete(ctx, client, initiativeID)
		if err != nil {
			return fmt.Errorf("Failed to delete initiative: %v", err)
		}

		if !resp.InitiativeDelete.Success {
			return errors.New("Failed to delete initiative")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.InitiativeDelete.EntityId})
		} else if plaintext {
			fmt.Printf("Deleted initiative %s\n", resp.InitiativeDelete.EntityId)
		} else {
			output.Success(fmt.Sprintf("Deleted initiative %s", resp.InitiativeDelete.EntityId), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeSetLeadCmd = &cobra.Command{
	Use:   "set-lead <initiative-id-or-name>",
	Short: "Set an initiative's lead team",
	Long: `Set (or clear) the team that leads an initiative.

Person ownership is handled by --owner on 'initiative create'/'initiative update', not here.

Examples:
  lincli initiative set-lead ID --team ENG
  lincli initiative set-lead ID --team ENG --mode includeDescendants
  lincli initiative set-lead ID --team none   # clear the lead team`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		team, _ := cmd.Flags().GetString("team")
		if team == "" {
			return errors.New("Team is required (--team)")
		}

		modeStr, _ := cmd.Flags().GetString("mode")
		var mode *api.InitiativeLeadTeamChangeMode
		switch modeStr {
		case "", "selectedOnly":
			val := api.InitiativeLeadTeamChangeModeSelectedonly
			mode = &val
		case "includeDescendants":
			val := api.InitiativeLeadTeamChangeModeIncludedescendants
			mode = &val
		default:
			return fmt.Errorf("Invalid mode '%s'. Valid values: selectedOnly, includeDescendants", modeStr)
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		var leadTeamID *string
		if strings.ToLower(team) == "none" {
			// leadTeamId is a top-level operation variable (not a nested
			// input field). A plain nil pointer gets stripped out of the
			// request entirely by the client's null-stripping (see
			// stripNulls in client.go), which Linear's API then treats as
			// "argument not provided" rather than "explicitly null" -
			// confirmed live: that omission fails with "leadTeamId
			// required". The NullSentinel forces an explicit JSON null so
			// the lead team is actually cleared.
			nullVal := api.NullSentinel
			leadTeamID = &nullVal
		} else {
			id, err := resolveTeam(ctx, client, cache, team)
			if err != nil {
				return fmt.Errorf("Failed to find team '%s': %v", team, err)
			}
			leadTeamID = &id
		}

		resp, err := api.InitiativeLeadTeamUpdate(ctx, client, initiativeID, leadTeamID, mode)
		if err != nil {
			return fmt.Errorf("Failed to update lead team: %v", err)
		}

		if !resp.InitiativeLeadTeamUpdate.Success {
			return errors.New("Failed to update lead team")
		}

		initiative := resp.InitiativeLeadTeamUpdate.Initiative

		if jsonOut {
			output.JSON(initiative)
		} else if plaintext {
			fmt.Printf("Updated lead team for initiative %s\n", initiative.InitiativeListFields.Name)
		} else {
			output.Success(fmt.Sprintf("Updated lead team for initiative %s", initiative.InitiativeListFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

// findInitiativeToProjectLink locates the InitiativeToProject join record
// linking projectID to initiativeID.
//
// Initiative has no field exposing these join records directly (only the
// raw `projects` connection, which is what InitiativeDetailFields uses for
// display). Project does expose them, via `initiativeToProjects`, and since
// a project can only belong to one initiative in a hierarchy (enforced by
// Linear), this returns at most one relevant link - confirmed live.
func findInitiativeToProjectLink(ctx context.Context, client graphql.Client, initiativeID, projectID string) (string, error) {
	resp, err := api.GetProjectInitiativeLinks(ctx, client, projectID)
	if err != nil {
		return "", fmt.Errorf("failed to look up project-initiative links: %w", err)
	}
	if resp.Project == nil || resp.Project.InitiativeToProjects == nil {
		return "", fmt.Errorf("project is not linked to any initiative")
	}
	for _, node := range resp.Project.InitiativeToProjects.Nodes {
		if node.Initiative != nil && node.Initiative.Id == initiativeID {
			return node.Id, nil
		}
	}
	return "", fmt.Errorf("project is not linked to this initiative")
}

var initiativeProjectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage an initiative's project links",
	Long: `Add, remove, and reorder the projects associated with an initiative.

Examples:
  lincli initiative project add "Q3 Growth" lincli-sandbox
  lincli initiative project remove "Q3 Growth" lincli-sandbox
  lincli initiative project reorder "Q3 Growth" lincli-sandbox --sort-order 50`,
}

var initiativeProjectAddCmd = &cobra.Command{
	Use:   "add <initiative-id-or-name> <project-id-or-name>",
	Short: "Associate a project with an initiative",
	Long:  `Associate a project with an initiative. A project can only belong to one initiative hierarchy at a time.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}
		projectID, err := resolveProject(ctx, client, cache, args[1])
		if err != nil {
			return err
		}

		input := api.InitiativeToProjectCreateInput{
			InitiativeId: initiativeID,
			ProjectId:    projectID,
		}

		resp, err := api.InitiativeToProjectCreate(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to add project to initiative: %v", err)
		}
		if !resp.InitiativeToProjectCreate.Success {
			return errors.New("Failed to add project to initiative")
		}

		link := resp.InitiativeToProjectCreate.InitiativeToProject

		if jsonOut {
			output.JSON(link)
		} else if plaintext {
			fmt.Printf("Added project %s to initiative %s\n", link.Project.Name, link.Initiative.Name)
		} else {
			output.Success(fmt.Sprintf("Added project %s to initiative %s", link.Project.Name, link.Initiative.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeProjectRemoveCmd = &cobra.Command{
	Use:     "remove <initiative-id-or-name> <project-id-or-name>",
	Aliases: []string{"rm"},
	Short:   "Remove a project from an initiative",
	Long:    `Remove a project from an initiative. This action executes immediately with no confirmation prompt.`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}
		projectID, err := resolveProject(ctx, client, cache, args[1])
		if err != nil {
			return err
		}

		linkID, err := findInitiativeToProjectLink(ctx, client, initiativeID, projectID)
		if err != nil {
			return err
		}

		resp, err := api.InitiativeToProjectDelete(ctx, client, linkID)
		if err != nil {
			return fmt.Errorf("Failed to remove project from initiative: %v", err)
		}
		if !resp.InitiativeToProjectDelete.Success {
			return errors.New("Failed to remove project from initiative")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.InitiativeToProjectDelete.EntityId})
		} else if plaintext {
			fmt.Printf("Removed project from initiative\n")
		} else {
			output.Success("Removed project from initiative", plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeProjectReorderCmd = &cobra.Command{
	Use:   "reorder <initiative-id-or-name> <project-id-or-name>",
	Short: "Change a project's sort order within an initiative",
	Long:  `Change the sort order of a project within an initiative's project list.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		if !cmd.Flags().Changed("sort-order") {
			return errors.New("Sort order is required (--sort-order)")
		}
		sortOrder, _ := cmd.Flags().GetFloat64("sort-order")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}
		projectID, err := resolveProject(ctx, client, cache, args[1])
		if err != nil {
			return err
		}

		linkID, err := findInitiativeToProjectLink(ctx, client, initiativeID, projectID)
		if err != nil {
			return err
		}

		input := api.InitiativeToProjectUpdateInput{SortOrder: &sortOrder}
		resp, err := api.InitiativeToProjectUpdate(ctx, client, linkID, &input)
		if err != nil {
			return fmt.Errorf("Failed to reorder project: %v", err)
		}
		if !resp.InitiativeToProjectUpdate.Success {
			return errors.New("Failed to reorder project")
		}

		link := resp.InitiativeToProjectUpdate.InitiativeToProject

		if jsonOut {
			output.JSON(link)
		} else if plaintext {
			fmt.Printf("Reordered project %s to sort order %s\n", link.Project.Name, link.SortOrder)
		} else {
			output.Success(fmt.Sprintf("Reordered project %s to sort order %s", link.Project.Name, link.SortOrder), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeLabelCmd = &cobra.Command{
	Use:   "label",
	Short: "Manage an initiative's labels",
	Long: `Add and remove labels on an initiative.

Initiative labels are a separate catalog from issue labels (a distinct
InitiativeLabel entity in Linear's schema, with its own names/ids) and are
gated behind a workspace-level feature; on workspaces where the feature
isn't enabled, these commands fail with a clear message from Linear.

Examples:
  lincli initiative label add "Q3 Growth" "Strategic"
  lincli initiative label remove "Q3 Growth" "Strategic"`,
}

var initiativeLabelAddCmd = &cobra.Command{
	Use:   "add <initiative-id-or-name> <label-name-or-id>",
	Short: "Add a label to an initiative",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}
		labelID, err := resolveInitiativeLabel(ctx, client, cache, args[1])
		if err != nil {
			return err
		}

		resp, err := api.InitiativeAddLabel(ctx, client, initiativeID, labelID)
		if err != nil {
			return fmt.Errorf("Failed to add label to initiative: %v", err)
		}
		if !resp.InitiativeAddLabel.Success {
			return errors.New("Failed to add label to initiative")
		}

		initiative := resp.InitiativeAddLabel.Initiative

		if jsonOut {
			output.JSON(initiative)
		} else if plaintext {
			fmt.Printf("Added label to initiative %s\n", initiative.InitiativeListFields.Name)
		} else {
			output.Success(fmt.Sprintf("Added label to initiative %s", initiative.InitiativeListFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeLabelRemoveCmd = &cobra.Command{
	Use:     "remove <initiative-id-or-name> <label-name-or-id>",
	Aliases: []string{"rm"},
	Short:   "Remove a label from an initiative",
	Long:    `Remove a label from an initiative. This action executes immediately with no confirmation prompt.`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}
		labelID, err := resolveInitiativeLabel(ctx, client, cache, args[1])
		if err != nil {
			return err
		}

		resp, err := api.InitiativeRemoveLabel(ctx, client, initiativeID, labelID)
		if err != nil {
			return fmt.Errorf("Failed to remove label from initiative: %v", err)
		}
		if !resp.InitiativeRemoveLabel.Success {
			return errors.New("Failed to remove label from initiative")
		}

		initiative := resp.InitiativeRemoveLabel.Initiative

		if jsonOut {
			output.JSON(initiative)
		} else if plaintext {
			fmt.Printf("Removed label from initiative %s\n", initiative.InitiativeListFields.Name)
		} else {
			output.Success(fmt.Sprintf("Removed label from initiative %s", initiative.InitiativeListFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeRelateCmd = &cobra.Command{
	Use:   "relate <initiative-id-or-name> <related-initiative-id-or-name>",
	Short: "Make one initiative a sub-initiative of another",
	Long: `Create a parent-child hierarchy relation between two initiatives:
<initiative> becomes the parent and <related-initiative> becomes the child.

Unlike issue relations, there is no relation-type flag here - Linear models
initiative relations purely as a parent/child hierarchy (InitiativeRelation
has no "type" field, verified against the schema). Creating a relation also
requires the Linear Enterprise plan (sub-initiatives); on other plans this
fails with a clear message from Linear.

Examples:
  lincli initiative relate "Platform" "Platform: Auth"
  lincli initiative relate "Platform" "Platform: Auth" --sort-order 10`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		parentID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}
		childID, err := resolveInitiative(ctx, client, cache, args[1])
		if err != nil {
			return err
		}

		input := api.InitiativeRelationCreateInput{
			InitiativeId:        parentID,
			RelatedInitiativeId: childID,
		}
		if cmd.Flags().Changed("sort-order") {
			sortOrder, _ := cmd.Flags().GetFloat64("sort-order")
			input.SortOrder = &sortOrder
		}

		resp, err := api.InitiativeRelationCreate(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to create initiative relation: %v", err)
		}
		if !resp.InitiativeRelationCreate.Success {
			return errors.New("Failed to create initiative relation")
		}

		rel := resp.InitiativeRelationCreate.InitiativeRelation

		if jsonOut {
			output.JSON(rel)
		} else if plaintext {
			fmt.Printf("Related %s as a sub-initiative of %s\n", rel.RelatedInitiative.Name, rel.Initiative.Name)
		} else {
			output.Success(fmt.Sprintf("Related %s as a sub-initiative of %s", rel.RelatedInitiative.Name, rel.Initiative.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeUnrelateCmd = &cobra.Command{
	Use:   "unrelate <initiative-id-or-name> <related-initiative-id-or-name>",
	Short: "Remove a parent-child relation between two initiatives",
	Long:  `Remove a parent-child hierarchy relation between two initiatives. This action executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		parentID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}
		childID, err := resolveInitiative(ctx, client, cache, args[1])
		if err != nil {
			return err
		}

		// initiativeRelations has no filter argument, so finding the join id
		// for a specific (initiative, relatedInitiative) pair means
		// paginating the workspace's relations client-side. 250 comfortably
		// covers typical workspace sizes; this is a known limitation for
		// very large workspaces (see pkg/api/operations/initiatives.graphql).
		limit := 250
		relID := ""
		for after := (*string)(nil); ; {
			resp, err := api.ListInitiativeRelations(ctx, client, &limit, after)
			if err != nil {
				return fmt.Errorf("Failed to look up initiative relations: %v", err)
			}
			for _, node := range resp.InitiativeRelations.Nodes {
				if node.Initiative != nil && node.RelatedInitiative != nil &&
					node.Initiative.Id == parentID && node.RelatedInitiative.Id == childID {
					relID = node.Id
					break
				}
			}
			if relID != "" || resp.InitiativeRelations.PageInfo == nil || !resp.InitiativeRelations.PageInfo.HasNextPage {
				break
			}
			after = resp.InitiativeRelations.PageInfo.EndCursor
		}

		if relID == "" {
			return errors.New("No relation found between these initiatives")
		}

		resp, err := api.InitiativeRelationDelete(ctx, client, relID)
		if err != nil {
			return fmt.Errorf("Failed to remove initiative relation: %v", err)
		}
		if !resp.InitiativeRelationDelete.Success {
			return errors.New("Failed to remove initiative relation")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.InitiativeRelationDelete.EntityId})
		} else if plaintext {
			fmt.Printf("Removed initiative relation\n")
		} else {
			output.Success("Removed initiative relation", plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeUpdatePostCmd = &cobra.Command{
	Use:   "update-post",
	Short: "Manage initiative status-update posts",
	Long: `List, create, edit, archive, and unarchive status updates posted to an
initiative.

Kept distinct from 'initiative update' (which edits the initiative record
itself - name, description, owner, status, etc): these are point-in-time
status posts shown on the initiative's timeline, each capturing a health
snapshot (onTrack/atRisk/offTrack).

Examples:
  lincli initiative update-post list "Q3 Growth"
  lincli initiative update-post create "Q3 Growth" --body "Kicking off" --health onTrack
  lincli initiative update-post edit UPDATE-ID --body "Revised" --health atRisk
  lincli initiative update-post archive UPDATE-ID`,
}

var initiativeUpdatePostListCmd = &cobra.Command{
	Use:     "list <initiative-id-or-name>",
	Aliases: []string{"ls"},
	Short:   "List status updates posted to an initiative",
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

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		filter := &api.InitiativeUpdateFilter{
			Initiative: &api.InitiativeFilter{
				Id: &api.IDComparator{Eq: &initiativeID},
			},
		}

		resp, err := api.ListInitiativeUpdates(ctx, client, filter, limitPtr, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list initiative updates: %v", err)
		}

		updates := resp.InitiativeUpdates.Nodes

		if len(updates) == 0 {
			output.Info("No status updates found for this initiative", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(updates)
			return nil
		}

		if plaintext {
			fmt.Println("# Initiative Status Updates")
			for _, u := range updates {
				fmt.Printf("## %s\n", u.CreatedAt.Format("2006-01-02 15:04:05"))
				fmt.Printf("- **ID**: %s\n", u.Id)
				fmt.Printf("- **Health**: %s\n", u.Health)
				if u.User != nil {
					fmt.Printf("- **Author**: %s\n", u.User.Name)
				}
				if u.ArchivedAt != nil {
					fmt.Printf("- **Archived**: %s\n", u.ArchivedAt.Format("2006-01-02"))
				}
				fmt.Printf("\n%s\n\n", u.Body)
			}
			return nil
		}

		headers := []string{"ID", "Date", "Health", "Author", "Body"}
		rows := make([][]string, len(updates))
		for i, u := range updates {
			author := ""
			if u.User != nil {
				author = u.User.Name
			}
			rows[i] = []string{
				u.Id,
				u.CreatedAt.Format("2006-01-02 15:04"),
				string(u.Health),
				author,
				truncateString(u.Body, 50),
			}
		}
		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)
		return nil
	},
}

var initiativeUpdatePostCreateCmd = &cobra.Command{
	Use:     "create <initiative-id-or-name>",
	Aliases: []string{"add", "new"},
	Short:   "Post a new status update to an initiative",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		body, _ := cmd.Flags().GetString("body")
		if body == "" {
			return errors.New("Body is required (--body)")
		}

		var health *api.InitiativeUpdateHealthType
		if healthStr, _ := cmd.Flags().GetString("health"); healthStr != "" {
			h, err := parseInitiativeUpdateHealth(healthStr)
			if err != nil {
				return err
			}
			health = &h
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		input := api.InitiativeUpdateCreateInput{
			InitiativeId: initiativeID,
			Body:         &body,
			Health:       health,
		}

		resp, err := api.CreateInitiativeUpdate(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to post initiative update: %v", err)
		}
		if !resp.InitiativeUpdateCreate.Success {
			return errors.New("Failed to post initiative update")
		}

		update := resp.InitiativeUpdateCreate.InitiativeUpdate

		if jsonOut {
			output.JSON(update)
		} else if plaintext {
			fmt.Printf("Posted status update %s\n", update.Id)
		} else {
			output.Success(fmt.Sprintf("Posted status update %s", update.Id), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeUpdatePostEditCmd = &cobra.Command{
	Use:   "edit <update-id>",
	Short: "Edit a status update's body or health",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		updateID := args[0]

		input := api.InitiativeUpdateUpdateInput{}
		if cmd.Flags().Changed("body") {
			body, _ := cmd.Flags().GetString("body")
			input.Body = &body
		}
		if cmd.Flags().Changed("health") {
			healthStr, _ := cmd.Flags().GetString("health")
			h, err := parseInitiativeUpdateHealth(healthStr)
			if err != nil {
				return err
			}
			input.Health = &h
		}
		if input.Body == nil && input.Health == nil {
			return errors.New("No updates specified. Use --body and/or --health.")
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.UpdateInitiativeUpdate(ctx, client, updateID, &input)
		if err != nil {
			return fmt.Errorf("Failed to edit status update: %v", err)
		}
		if !resp.InitiativeUpdateUpdate.Success {
			return errors.New("Failed to edit status update")
		}

		update := resp.InitiativeUpdateUpdate.InitiativeUpdate

		if jsonOut {
			output.JSON(update)
		} else if plaintext {
			fmt.Printf("Updated status update %s\n", update.Id)
		} else {
			output.Success(fmt.Sprintf("Updated status update %s", update.Id), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeUpdatePostArchiveCmd = &cobra.Command{
	Use:   "archive <update-id>",
	Short: "Archive a status update",
	Long:  `Archive a status update. This action executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		updateID := args[0]

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.InitiativeUpdateArchive(ctx, client, updateID)
		if err != nil {
			return fmt.Errorf("Failed to archive status update: %v", err)
		}
		if !resp.InitiativeUpdateArchive.Success {
			return errors.New("Failed to archive status update")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": updateID})
		} else if plaintext {
			fmt.Printf("Archived status update %s\n", updateID)
		} else {
			output.Success(fmt.Sprintf("Archived status update %s", updateID), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeUpdatePostUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <update-id>",
	Short: "Unarchive a status update",
	Long:  `Unarchive a previously archived status update. This action executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		updateID := args[0]

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.InitiativeUpdateUnarchive(ctx, client, updateID)
		if err != nil {
			return fmt.Errorf("Failed to unarchive status update: %v", err)
		}
		if !resp.InitiativeUpdateUnarchive.Success {
			return errors.New("Failed to unarchive status update")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": updateID})
		} else if plaintext {
			fmt.Printf("Unarchived status update %s\n", updateID)
		} else {
			output.Success(fmt.Sprintf("Unarchived status update %s", updateID), plaintext, jsonOut)
		}
		return nil
	},
}

var initiativeUpdateReminderCmd = &cobra.Command{
	Use:   "update-reminder <initiative-id-or-name>",
	Short: "Notify a user to post an initiative status update",
	Long: `Send a notification reminding a user to post a status update for an
initiative. Defaults to the initiative's owner if --user is not given.

Examples:
  lincli initiative update-reminder "Q3 Growth"
  lincli initiative update-reminder "Q3 Growth" --user jane@example.com`,
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

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		var userID *string
		if user, _ := cmd.Flags().GetString("user"); user != "" {
			id, err := resolveUser(ctx, client, cache, user)
			if err != nil {
				return fmt.Errorf("Failed to find user '%s': %v", user, err)
			}
			userID = &id
		}

		resp, err := api.CreateInitiativeUpdateReminder(ctx, client, initiativeID, userID)
		if err != nil {
			return fmt.Errorf("Failed to send update reminder: %v", err)
		}
		if !resp.CreateInitiativeUpdateReminder.Success {
			return errors.New("Failed to send update reminder")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true})
		} else if plaintext {
			fmt.Println("Sent update reminder")
		} else {
			output.Success("Sent update reminder", plaintext, jsonOut)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initiativeCmd)
	initiativeCmd.AddCommand(initiativeListCmd)
	initiativeCmd.AddCommand(initiativeGetCmd)
	initiativeCmd.AddCommand(initiativeCreateCmd)
	initiativeCmd.AddCommand(initiativeUpdateCmd)
	initiativeCmd.AddCommand(initiativeArchiveCmd)
	initiativeCmd.AddCommand(initiativeUnarchiveCmd)
	initiativeCmd.AddCommand(initiativeDeleteCmd)
	initiativeCmd.AddCommand(initiativeSetLeadCmd)

	// Project links
	initiativeCmd.AddCommand(initiativeProjectCmd)
	initiativeProjectCmd.AddCommand(initiativeProjectAddCmd)
	initiativeProjectCmd.AddCommand(initiativeProjectRemoveCmd)
	initiativeProjectCmd.AddCommand(initiativeProjectReorderCmd)

	// Labels
	initiativeCmd.AddCommand(initiativeLabelCmd)
	initiativeLabelCmd.AddCommand(initiativeLabelAddCmd)
	initiativeLabelCmd.AddCommand(initiativeLabelRemoveCmd)

	// Relations
	initiativeCmd.AddCommand(initiativeRelateCmd)
	initiativeCmd.AddCommand(initiativeUnrelateCmd)

	// Status-update posts
	initiativeCmd.AddCommand(initiativeUpdatePostCmd)
	initiativeUpdatePostCmd.AddCommand(initiativeUpdatePostListCmd)
	initiativeUpdatePostCmd.AddCommand(initiativeUpdatePostCreateCmd)
	initiativeUpdatePostCmd.AddCommand(initiativeUpdatePostEditCmd)
	initiativeUpdatePostCmd.AddCommand(initiativeUpdatePostArchiveCmd)
	initiativeUpdatePostCmd.AddCommand(initiativeUpdatePostUnarchiveCmd)

	// Update reminder
	initiativeCmd.AddCommand(initiativeUpdateReminderCmd)

	// List command flags. Initiatives are long-lived strategic groupings, so
	// (unlike issues/projects) the default time window is all_time rather
	// than the usual 6-month lookback.
	initiativeListCmd.Flags().IntP("limit", "l", 50, "Maximum number of initiatives to return")
	initiativeListCmd.Flags().StringP("sort", "o", "linear", "Sort order: linear (default), created, updated")
	initiativeListCmd.Flags().StringP("newer-than", "n", "all_time", "Show initiatives created after this time (default: all_time)")
	initiativeListCmd.Flags().Bool("include-archived", false, "Include archived initiatives")

	// Create command flags
	initiativeCreateCmd.Flags().String("name", "", "Initiative name (required)")
	initiativeCreateCmd.Flags().String("description", "", "Initiative description")
	initiativeCreateCmd.Flags().String("owner", "", "Owner email, name, 'me', or user ID")
	initiativeCreateCmd.Flags().String("target-date", "", "Target completion date (YYYY-MM-DD)")
	initiativeCreateCmd.Flags().String("status", "", fmt.Sprintf("Initiative status: %s", strings.Join(initiativeStatusValues, ", ")))

	// Update command flags
	initiativeUpdateCmd.Flags().String("name", "", "New initiative name")
	initiativeUpdateCmd.Flags().String("description", "", "New description")
	initiativeUpdateCmd.Flags().String("owner", "", "New owner (email, name, 'me', user ID, or 'unassigned' to clear)")
	initiativeUpdateCmd.Flags().String("target-date", "", "New target completion date (YYYY-MM-DD, or empty string to clear)")
	initiativeUpdateCmd.Flags().String("status", "", fmt.Sprintf("New status: %s", strings.Join(initiativeStatusValues, ", ")))

	// Set-lead command flags
	initiativeSetLeadCmd.Flags().StringP("team", "t", "", "Team key, name, or ID to lead the initiative, or 'none' to clear (required)")
	initiativeSetLeadCmd.Flags().String("mode", "selectedOnly", "How the change applies: selectedOnly (default) or includeDescendants")

	// Project reorder command flags
	initiativeProjectReorderCmd.Flags().Float64("sort-order", 0, "New sort order for the project within the initiative (required)")

	// Relate command flags
	initiativeRelateCmd.Flags().Float64("sort-order", 0, "Sort order of the child initiative within its parent")

	// Update-post list command flags
	initiativeUpdatePostListCmd.Flags().IntP("limit", "l", 50, "Maximum number of status updates to return")

	// Update-post create command flags
	initiativeUpdatePostCreateCmd.Flags().String("body", "", "Status update body in markdown (required)")
	initiativeUpdatePostCreateCmd.Flags().String("health", "", fmt.Sprintf("Initiative health at the time of the update: %s", strings.Join(initiativeUpdateHealthValues, ", ")))
	_ = initiativeUpdatePostCreateCmd.MarkFlagRequired("body")

	// Update-post edit command flags
	initiativeUpdatePostEditCmd.Flags().String("body", "", "New status update body in markdown")
	initiativeUpdatePostEditCmd.Flags().String("health", "", fmt.Sprintf("New health value: %s", strings.Join(initiativeUpdateHealthValues, ", ")))

	// Update-reminder command flags
	initiativeUpdateReminderCmd.Flags().String("user", "", "User email, name, 'me', or ID to remind (defaults to the initiative owner)")
}

// buildInitiativeFilterTyped builds a typed InitiativeFilter from command flags.
func buildInitiativeFilterTyped(cmd *cobra.Command) (api.InitiativeFilter, error) {
	filter := api.InitiativeFilter{}

	newerThan, _ := cmd.Flags().GetString("newer-than")
	createdAt, err := utils.ParseTimeExpression(newerThan)
	if err != nil {
		return filter, fmt.Errorf("Invalid newer-than value: %v", err)
	}
	if createdAt != "" {
		filter.CreatedAt = &api.DateComparator{
			Gte: &createdAt,
		}
	}

	return filter, nil
}
