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
  lincli initiative set-lead ID --team ENG`,
}

var initiativeListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List initiatives",
	Long:    `List all initiatives in your Linear workspace.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
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
				output.Error(fmt.Sprintf("Invalid sort option: %s. Valid options are: linear, created, updated", sortBy), plaintext, jsonOut)
				os.Exit(1)
			}
		}

		filter := buildInitiativeFilterTyped(cmd, plaintext, jsonOut)

		includeArchived, _ := cmd.Flags().GetBool("include-archived")
		includeArchivedPtr := &includeArchived

		resp, err := api.ListInitiatives(ctx, client, &filter, limitPtr, nil, orderByEnum, includeArchivedPtr)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list initiatives: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if len(resp.Initiatives.Nodes) == 0 {
			output.Info("No initiatives found", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(resp.Initiatives.Nodes)
			return
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
			return
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
	},
}

var initiativeGetCmd = &cobra.Command{
	Use:     "get <initiative-id-or-name>",
	Aliases: []string{"show"},
	Short:   "Get initiative details",
	Long:    `Get detailed information about a specific initiative. Accepts an initiative ID or an exact (case-insensitive) name.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.GetInitiative(ctx, client, initiativeID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get initiative: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		f := resp.Initiative.InitiativeDetailFields

		if jsonOut {
			output.JSON(f)
			return
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

			// Lead team. Project links, labels, relations, and status updates
			// are intentionally left out here; they arrive in a later pass.
			if f.LeadTeam != nil {
				fmt.Printf("\n## Lead Team\n")
				fmt.Printf("- **%s** (%s)\n", f.LeadTeam.Name, f.LeadTeam.Key)
			}

			fmt.Printf("\n## URL\n- %s\n", f.Url)
			return
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

		if f.Url != "" {
			fmt.Printf("\n%s %s\n",
				color.New(color.Bold).Sprint("URL:"),
				color.New(color.FgBlue, color.Underline).Sprint(f.Url))
		}

		fmt.Println()
	},
}

var initiativeCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a new initiative",
	Long:    `Create a new initiative in Linear.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()
		cache := newResolverCache()

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			output.Error("Name is required (--name)", plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.InitiativeCreateInput{Name: name}

		if description, _ := cmd.Flags().GetString("description"); description != "" {
			input.Description = &description
		}

		if owner, _ := cmd.Flags().GetString("owner"); owner != "" {
			ownerID, err := resolveUser(ctx, client, cache, owner)
			if err != nil {
				output.Error(fmt.Sprintf("Failed to find owner '%s': %v", owner, err), plaintext, jsonOut)
				os.Exit(1)
			}
			input.OwnerId = &ownerID
		}

		if targetDate, _ := cmd.Flags().GetString("target-date"); targetDate != "" {
			input.TargetDate = &targetDate
		}

		if status, _ := cmd.Flags().GetString("status"); status != "" {
			statusEnum, err := parseInitiativeStatus(status)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.Status = &statusEnum
		}

		createResp, err := api.CreateInitiative(ctx, client, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create initiative: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if !createResp.InitiativeCreate.Success {
			output.Error("Failed to create initiative", plaintext, jsonOut)
			os.Exit(1)
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
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
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
					output.Error(fmt.Sprintf("Failed to find owner '%s': %v", owner, err), plaintext, jsonOut)
					os.Exit(1)
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
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.Status = &statusEnum
		}

		hasUpdates := input.Name != nil ||
			input.Description != nil ||
			cmd.Flags().Changed("owner") ||
			cmd.Flags().Changed("target-date") ||
			input.Status != nil

		if !hasUpdates {
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		updateResp, err := api.UpdateInitiative(ctx, client, initiativeID, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update initiative: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if !updateResp.InitiativeUpdate.Success {
			output.Error("Failed to update initiative", plaintext, jsonOut)
			os.Exit(1)
		}

		updated := updateResp.InitiativeUpdate.Initiative

		if jsonOut {
			output.JSON(updated)
		} else if plaintext {
			fmt.Printf("Updated initiative %s\n", updated.InitiativeListFields.Name)
		} else {
			output.Success(fmt.Sprintf("Updated initiative %s", updated.InitiativeListFields.Name), plaintext, jsonOut)
		}
	},
}

var initiativeArchiveCmd = &cobra.Command{
	Use:   "archive <initiative-id-or-name>",
	Short: "Archive an initiative",
	Long:  `Archive an initiative. This action executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.InitiativeArchive(ctx, client, initiativeID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to archive initiative: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if !resp.InitiativeArchive.Success {
			output.Error("Failed to archive initiative", plaintext, jsonOut)
			os.Exit(1)
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
	},
}

var initiativeUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <initiative-id-or-name>",
	Short: "Unarchive an initiative",
	Long:  `Unarchive a previously archived initiative. This action executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.InitiativeUnarchive(ctx, client, initiativeID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to unarchive initiative: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if !resp.InitiativeUnarchive.Success {
			output.Error("Failed to unarchive initiative", plaintext, jsonOut)
			os.Exit(1)
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
	},
}

var initiativeDeleteCmd = &cobra.Command{
	Use:   "delete <initiative-id-or-name>",
	Short: "Delete an initiative",
	Long:  `Delete (trash) an initiative. This action executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()
		cache := newResolverCache()

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.InitiativeDelete(ctx, client, initiativeID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete initiative: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if !resp.InitiativeDelete.Success {
			output.Error("Failed to delete initiative", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.InitiativeDelete.EntityId})
		} else if plaintext {
			fmt.Printf("Deleted initiative %s\n", resp.InitiativeDelete.EntityId)
		} else {
			output.Success(fmt.Sprintf("Deleted initiative %s", resp.InitiativeDelete.EntityId), plaintext, jsonOut)
		}
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
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		team, _ := cmd.Flags().GetString("team")
		if team == "" {
			output.Error("Team is required (--team)", plaintext, jsonOut)
			os.Exit(1)
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
			output.Error(fmt.Sprintf("Invalid mode '%s'. Valid values: selectedOnly, includeDescendants", modeStr), plaintext, jsonOut)
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

		initiativeID, err := resolveInitiative(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
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
				output.Error(fmt.Sprintf("Failed to find team '%s': %v", team, err), plaintext, jsonOut)
				os.Exit(1)
			}
			leadTeamID = &id
		}

		resp, err := api.InitiativeLeadTeamUpdate(ctx, client, initiativeID, leadTeamID, mode)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update lead team: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if !resp.InitiativeLeadTeamUpdate.Success {
			output.Error("Failed to update lead team", plaintext, jsonOut)
			os.Exit(1)
		}

		initiative := resp.InitiativeLeadTeamUpdate.Initiative

		if jsonOut {
			output.JSON(initiative)
		} else if plaintext {
			fmt.Printf("Updated lead team for initiative %s\n", initiative.InitiativeListFields.Name)
		} else {
			output.Success(fmt.Sprintf("Updated lead team for initiative %s", initiative.InitiativeListFields.Name), plaintext, jsonOut)
		}
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
}

// buildInitiativeFilterTyped builds a typed InitiativeFilter from command flags.
func buildInitiativeFilterTyped(cmd *cobra.Command, plaintext, jsonOut bool) api.InitiativeFilter {
	filter := api.InitiativeFilter{}

	newerThan, _ := cmd.Flags().GetString("newer-than")
	createdAt, err := utils.ParseTimeExpression(newerThan)
	if err != nil {
		output.Error(fmt.Sprintf("Invalid newer-than value: %v", err), plaintext, jsonOut)
		os.Exit(1)
	}
	if createdAt != "" {
		filter.CreatedAt = &api.DateComparator{
			Gte: &createdAt,
		}
	}

	return filter
}
