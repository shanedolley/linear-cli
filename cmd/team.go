package cmd

import (
	"context"
	"errors"
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

// teamCmd represents the team command
var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage Linear teams",
	Long: `Manage Linear teams including listing teams, viewing team details, and listing team members.

Examples:
  lincli team list              # List all teams
  lincli team get ENG           # Get team details
  lincli team members ENG       # List team members`,
}

var teamListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List teams",
	Long:    `List all teams in your Linear workspace.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		// Get limit
		limit, _ := cmd.Flags().GetInt("limit")

		// Get sort option
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

		// Convert limit to pointer
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		// Get teams
		resp, err := api.ListTeams(context.Background(), client, limitPtr, nil, orderByEnum)
		if err != nil {
			return fmt.Errorf("Failed to list teams: %v", err)
		}

		// Handle output
		if jsonOut {
			output.JSON(resp.Teams.Nodes)
		} else if plaintext {
			fmt.Println("Key\tName\tDescription\tPrivate\tIssues")
			for _, node := range resp.Teams.Nodes {
				f := node.TeamListFields
				description := ""
				if f.Description != nil {
					description = *f.Description
				}
				if len(description) > 50 {
					description = description[:47] + "..."
				}
				fmt.Printf("%s\t%s\t%s\t%v\t%d\n",
					f.Key,
					f.Name,
					description,
					f.Private,
					f.IssueCount,
				)
			}
		} else {
			// Table output
			headers := []string{"Key", "Name", "Description", "Private", "Issues"}
			rows := [][]string{}

			for _, node := range resp.Teams.Nodes {
				f := node.TeamListFields
				description := ""
				if f.Description != nil {
					description = *f.Description
				}
				if len(description) > 40 {
					description = description[:37] + "..."
				}

				privateStr := ""
				if f.Private {
					privateStr = color.New(color.FgYellow).Sprint("🔒 Yes")
				} else {
					privateStr = color.New(color.FgGreen).Sprint("No")
				}

				rows = append(rows, []string{
					color.New(color.FgCyan, color.Bold).Sprint(f.Key),
					f.Name,
					description,
					privateStr,
					fmt.Sprintf("%d", f.IssueCount),
				})
			}

			output.Table(output.TableData{
				Headers: headers,
				Rows:    rows,
			}, plaintext, jsonOut)

			if !plaintext && !jsonOut {
				fmt.Printf("\n%s %d teams\n",
					color.New(color.FgGreen).Sprint("✓"),
					len(resp.Teams.Nodes))
			}
		}
		return nil
	},
}

var teamGetCmd = &cobra.Command{
	Use:     "get TEAM-KEY",
	Aliases: []string{"show"},
	Short:   "Get team details",
	Long:    `Get detailed information about a specific team.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		teamKey := args[0]

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		// Get team details
		resp, err := api.GetTeam(context.Background(), client, teamKey)
		if err != nil {
			return fmt.Errorf("Failed to get team: %v", err)
		}
		team := resp.Team.TeamDetailFields

		// Handle output
		if jsonOut {
			output.JSON(team)
		} else if plaintext {
			fmt.Printf("Key: %s\n", team.Key)
			fmt.Printf("Name: %s\n", team.Name)
			if team.Description != nil && *team.Description != "" {
				fmt.Printf("Description: %s\n", *team.Description)
			}
			fmt.Printf("Private: %v\n", team.Private)
			fmt.Printf("Issue Count: %d\n", team.IssueCount)
		} else {
			// Formatted output
			fmt.Println()
			fmt.Printf("%s %s (%s)\n",
				color.New(color.FgCyan, color.Bold).Sprint("👥 Team:"),
				team.Name,
				color.New(color.FgCyan).Sprint(team.Key))
			fmt.Println(strings.Repeat("─", 50))

			if team.Description != nil && *team.Description != "" {
				fmt.Printf("\n%s\n%s\n",
					color.New(color.Bold).Sprint("Description:"),
					*team.Description)
			}

			privateStr := color.New(color.FgGreen).Sprint("No")
			if team.Private {
				privateStr = color.New(color.FgYellow).Sprint("🔒 Yes")
			}
			fmt.Printf("\n%s %s\n", color.New(color.Bold).Sprint("Private:"), privateStr)
			fmt.Printf("%s %d\n", color.New(color.Bold).Sprint("Total Issues:"), team.IssueCount)
			fmt.Println()
		}
		return nil
	},
}

var teamMembersCmd = &cobra.Command{
	Use:   "members TEAM-KEY",
	Short: "List team members",
	Long:  `List all members of a specific team.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		teamKey := args[0]

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		// Get team members
		resp, err := api.GetTeamMembers(context.Background(), client, teamKey)
		if err != nil {
			return fmt.Errorf("Failed to get team members: %v", err)
		}
		members := resp.Team.Members.Nodes

		// Handle output
		if jsonOut {
			output.JSON(members)
		} else if plaintext {
			fmt.Println("Name\tEmail\tRole\tActive")
			for _, member := range members {
				role := "Member"
				if member.Admin {
					role = "Admin"
				}
				fmt.Printf("%s\t%s\t%s\t%v\n",
					member.Name,
					member.Email,
					role,
					member.Active,
				)
			}
		} else {
			// Table output
			headers := []string{"Name", "Email", "Role", "Status"}
			rows := [][]string{}

			for _, member := range members {
				role := "Member"
				roleColor := color.New(color.FgWhite)
				if member.Admin {
					role = "Admin"
					roleColor = color.New(color.FgYellow)
				}
				if member.IsMe {
					role = role + " (You)"
					roleColor = color.New(color.FgCyan, color.Bold)
				}

				status := color.New(color.FgGreen).Sprint("✓ Active")
				if !member.Active {
					status = color.New(color.FgRed).Sprint("✗ Inactive")
				}

				rows = append(rows, []string{
					member.Name,
					color.New(color.FgCyan).Sprint(member.Email),
					roleColor.Sprint(role),
					status,
				})
			}

			output.Table(output.TableData{
				Headers: headers,
				Rows:    rows,
			}, plaintext, jsonOut)

			if !plaintext && !jsonOut {
				fmt.Printf("\n%s %d members in team %s\n",
					color.New(color.FgGreen).Sprint("✓"),
					len(members),
					color.New(color.FgCyan).Sprint(teamKey))
			}
		}
		return nil
	},
}

var teamCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a team",
	Long:    `Create a new team. Without --key, Linear derives one from the name.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return errors.New("Name is required (--name)")
		}

		input := api.TeamCreateInput{Name: name}
		if key, _ := cmd.Flags().GetString("key"); key != "" {
			input.Key = &key
		}
		if description, _ := cmd.Flags().GetString("description"); description != "" {
			input.Description = &description
		}
		if color, _ := cmd.Flags().GetString("color"); color != "" {
			input.Color = &color
		}
		if icon, _ := cmd.Flags().GetString("icon"); icon != "" {
			input.Icon = &icon
		}
		if cmd.Flags().Changed("private") {
			private, _ := cmd.Flags().GetBool("private")
			input.Private = &private
		}

		resp, err := api.TeamCreate(context.Background(), client, &input)
		if err != nil {
			return fmt.Errorf("Failed to create team: %v", err)
		}
		if resp.TeamCreate == nil || !resp.TeamCreate.Success || resp.TeamCreate.Team == nil {
			return errors.New("Failed to create team")
		}

		if jsonOut {
			output.JSON(resp.TeamCreate.Team)
		} else {
			t := resp.TeamCreate.Team.TeamDetailFields
			output.Success(fmt.Sprintf("Created team %s (%s)", t.Name, t.Key), plaintext, jsonOut)
		}
		return nil
	},
}

var teamUpdateCmd = &cobra.Command{
	Use:   "update <team-key>",
	Short: "Update a team",
	Long:  `Update a team's name, key, description, privacy, color, or icon.`,
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

		teamID, err := resolveTeam(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		input := api.TeamUpdateInput{}
		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
		}
		if cmd.Flags().Changed("key") {
			key, _ := cmd.Flags().GetString("key")
			input.Key = &key
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
		}
		if cmd.Flags().Changed("color") {
			color, _ := cmd.Flags().GetString("color")
			input.Color = &color
		}
		if cmd.Flags().Changed("icon") {
			icon, _ := cmd.Flags().GetString("icon")
			input.Icon = &icon
		}
		if cmd.Flags().Changed("private") {
			private, _ := cmd.Flags().GetBool("private")
			input.Private = &private
		}

		if input.Name == nil && input.Key == nil && input.Description == nil && input.Color == nil && input.Icon == nil && input.Private == nil {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.TeamUpdate(ctx, client, teamID, &input)
		if err != nil {
			return fmt.Errorf("Failed to update team: %v", err)
		}
		if resp.TeamUpdate == nil || !resp.TeamUpdate.Success {
			return errors.New("Failed to update team")
		}

		if jsonOut {
			output.JSON(resp.TeamUpdate.Team)
		} else {
			output.Success(fmt.Sprintf("Updated team %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var teamDeleteCmd = &cobra.Command{
	Use:   "delete <team-key>",
	Short: "Delete a team",
	Long: `Delete a team. This action executes immediately with no confirmation
prompt and archives the team along with its issues. Reversible with
'team unarchive'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runTeamStateChange(cmd, args[0], true)
		return nil
	},
}

var teamUnarchiveCmd = &cobra.Command{
	Use:     "unarchive <team-key>",
	Aliases: []string{"restore"},
	Short:   "Restore an archived team",
	Long:    `Restore a previously archived team.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runTeamStateChange(cmd, args[0], false)
		return nil
	},
}

// runTeamStateChange handles delete/unarchive, which share the same shape:
// resolve the team, call the mutation, report. delete returns a DeletePayload
// (entityId); unarchive returns a TeamArchivePayload (entity), so the resolved
// entity ID is read from the matching field.
func runTeamStateChange(cmd *cobra.Command, teamRef string, del bool) {
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

	teamID, err := resolveTeam(ctx, client, cache, teamRef)
	if err != nil {
		output.Error(err.Error(), plaintext, jsonOut)
		os.Exit(1)
	}

	verb, pastVerb := "delete", "Deleted"
	if !del {
		verb, pastVerb = "unarchive", "Unarchived"
	}

	var success bool
	entityID := teamID
	if del {
		resp, err := api.TeamDelete(ctx, client, teamID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete team: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.TeamDelete != nil {
			success = resp.TeamDelete.Success
			entityID = resp.TeamDelete.EntityId
		}
	} else {
		resp, err := api.TeamUnarchive(ctx, client, teamID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to unarchive team: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.TeamUnarchive != nil {
			success = resp.TeamUnarchive.Success
			if resp.TeamUnarchive.Entity != nil {
				entityID = resp.TeamUnarchive.Entity.Id
			}
		}
	}

	if !success {
		output.Error(fmt.Sprintf("Failed to %s team", verb), plaintext, jsonOut)
		os.Exit(1)
	}

	if jsonOut {
		output.JSON(map[string]interface{}{"success": true, "id": entityID})
	} else {
		output.Success(fmt.Sprintf("%s team %s", pastVerb, teamRef), plaintext, jsonOut)
	}
}

var teamMemberCmd = &cobra.Command{
	Use:   "member",
	Short: "Manage team membership",
	Long:  `Add or remove users from a team.`,
}

var teamMemberAddCmd = &cobra.Command{
	Use:   "add <team-key> <user>",
	Short: "Add a user to a team",
	Long:  `Add a user (email, name, or 'me') to a team.`,
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

		teamID, err := resolveTeam(ctx, client, cache, args[0])
		if err != nil {
			return err
		}
		userID, err := resolveUser(ctx, client, cache, args[1])
		if err != nil {
			return err
		}

		input := api.TeamMembershipCreateInput{TeamId: teamID, UserId: userID}
		resp, err := api.TeamMembershipCreate(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to add member: %v", err)
		}
		if resp.TeamMembershipCreate == nil || !resp.TeamMembershipCreate.Success {
			return errors.New("Failed to add member")
		}

		if jsonOut {
			output.JSON(resp.TeamMembershipCreate.TeamMembership)
		} else {
			output.Success(fmt.Sprintf("Added %s to team %s", args[1], args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var teamMemberRemoveCmd = &cobra.Command{
	Use:     "remove <team-key> <user>",
	Aliases: []string{"rm"},
	Short:   "Remove a user from a team",
	Long:    `Remove a user (email, name, or 'me') from a team.`,
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

		membershipID, err := resolveTeamMembership(ctx, client, cache, args[0], args[1])
		if err != nil {
			return err
		}

		resp, err := api.TeamMembershipDelete(ctx, client, membershipID)
		if err != nil {
			return fmt.Errorf("Failed to remove member: %v", err)
		}
		if resp.TeamMembershipDelete == nil || !resp.TeamMembershipDelete.Success {
			return errors.New("Failed to remove member")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.TeamMembershipDelete.EntityId})
		} else {
			output.Success(fmt.Sprintf("Removed %s from team %s", args[1], args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var teamSetRoleCmd = &cobra.Command{
	Use:   "set-role <team-key> <user>",
	Short: "Set a user's team membership role",
	Long: `Set a user's role WITHIN A TEAM to owner or member. This is the team
membership role, not the organization role; use 'user set-role' to change a
user's org role (admin, guest, and so on).`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		role, _ := cmd.Flags().GetString("role")
		owner, err := parseTeamOwnerRole(role)
		if err != nil {
			return err
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		membershipID, err := resolveTeamMembership(ctx, client, cache, args[0], args[1])
		if err != nil {
			return err
		}

		input := api.TeamMembershipUpdateInput{Owner: &owner}
		resp, err := api.TeamMembershipUpdate(ctx, client, membershipID, &input)
		if err != nil {
			return fmt.Errorf("Failed to set role: %v", err)
		}
		if resp.TeamMembershipUpdate == nil || !resp.TeamMembershipUpdate.Success {
			return errors.New("Failed to set role")
		}

		if jsonOut {
			output.JSON(resp.TeamMembershipUpdate.TeamMembership)
		} else {
			output.Success(fmt.Sprintf("Set %s to %s in team %s", args[1], role, args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// parseTeamOwnerRole maps a team-membership role string to the owner boolean
// Linear uses. Only "owner" and "member" are valid (case-insensitive).
func parseTeamOwnerRole(role string) (bool, error) {
	switch strings.ToLower(role) {
	case "owner":
		return true, nil
	case "member":
		return false, nil
	default:
		return false, fmt.Errorf("invalid team role %q. Valid values: owner, member", role)
	}
}

func init() {
	rootCmd.AddCommand(teamCmd)
	teamCmd.AddCommand(teamListCmd)
	teamCmd.AddCommand(teamGetCmd)
	teamCmd.AddCommand(teamMembersCmd)
	teamCmd.AddCommand(teamCreateCmd)
	teamCmd.AddCommand(teamUpdateCmd)
	teamCmd.AddCommand(teamDeleteCmd)
	teamCmd.AddCommand(teamUnarchiveCmd)
	teamCmd.AddCommand(teamMemberCmd)
	teamMemberCmd.AddCommand(teamMemberAddCmd)
	teamMemberCmd.AddCommand(teamMemberRemoveCmd)
	teamCmd.AddCommand(teamSetRoleCmd)

	// List command flags
	teamListCmd.Flags().IntP("limit", "l", 50, "Maximum number of teams to return")
	teamListCmd.Flags().StringP("sort", "o", "linear", "Sort order: linear (default), created, updated")

	// Create command flags
	teamCreateCmd.Flags().String("name", "", "Team name (required)")
	teamCreateCmd.Flags().String("key", "", "Team key (e.g. ENG); derived from the name if omitted")
	teamCreateCmd.Flags().StringP("description", "d", "", "Team description")
	teamCreateCmd.Flags().Bool("private", false, "Make the team private")
	teamCreateCmd.Flags().String("color", "", "Team color as a HEX string (e.g. #EB5757)")
	teamCreateCmd.Flags().String("icon", "", "Team icon")

	// Update command flags
	teamUpdateCmd.Flags().String("name", "", "Team name")
	teamUpdateCmd.Flags().String("key", "", "Team key")
	teamUpdateCmd.Flags().StringP("description", "d", "", "Team description")
	teamUpdateCmd.Flags().Bool("private", false, "Whether the team is private")
	teamUpdateCmd.Flags().String("color", "", "Team color as a HEX string (e.g. #EB5757)")
	teamUpdateCmd.Flags().String("icon", "", "Team icon")

	// set-role flags
	teamSetRoleCmd.Flags().String("role", "", "Team membership role: owner or member (required)")
	_ = teamSetRoleCmd.MarkFlagRequired("role")
}
