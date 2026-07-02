package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/shanedolley/lincli/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// constructProjectURL constructs an ID-based project URL
func constructProjectURL(projectID string, originalURL string) string {
	// Extract workspace from the original URL
	// Format: https://linear.app/{workspace}/project/{slug}
	if originalURL == "" {
		return ""
	}

	parts := strings.Split(originalURL, "/")
	if len(parts) >= 5 {
		workspace := parts[3]
		return fmt.Sprintf("https://linear.app/%s/project/%s", workspace, projectID)
	}

	// Fallback to original URL if we can't parse it
	return originalURL
}

// projectUpdateHealthValues lists the real ProjectUpdateHealthType enum
// values (verified against schema.graphql), used both for --health
// validation on 'project update-post create/edit' and to build the "valid
// values" error message.
var projectUpdateHealthValues = []string{
	string(api.ProjectUpdateHealthTypeOntrack),
	string(api.ProjectUpdateHealthTypeAtrisk),
	string(api.ProjectUpdateHealthTypeOfftrack),
}

// parseProjectUpdateHealth validates a --health value against
// ProjectUpdateHealthType (case-insensitively) and returns the canonical
// enum value.
func parseProjectUpdateHealth(health string) (api.ProjectUpdateHealthType, error) {
	for _, v := range projectUpdateHealthValues {
		if strings.EqualFold(v, health) {
			return api.ProjectUpdateHealthType(v), nil
		}
	}
	return "", fmt.Errorf("Invalid health '%s'. Valid values: %s", health, strings.Join(projectUpdateHealthValues, ", "))
}

// projectStatusTypeValues lists the real ProjectStatusType enum values
// (verified against schema.graphql), used both for --type validation on
// 'project status create/update' and to build the "valid values" error
// message.
var projectStatusTypeValues = []string{
	string(api.ProjectStatusTypeBacklog),
	string(api.ProjectStatusTypePlanned),
	string(api.ProjectStatusTypeStarted),
	string(api.ProjectStatusTypePaused),
	string(api.ProjectStatusTypeCompleted),
	string(api.ProjectStatusTypeCanceled),
}

// parseProjectStatusType validates a --type value against ProjectStatusType
// (case-insensitively) and returns the canonical enum value.
func parseProjectStatusType(t string) (api.ProjectStatusType, error) {
	for _, v := range projectStatusTypeValues {
		if strings.EqualFold(v, t) {
			return api.ProjectStatusType(v), nil
		}
	}
	return "", fmt.Errorf("Invalid status type '%s'. Valid values: %s", t, strings.Join(projectStatusTypeValues, ", "))
}

// projectRelationTypeValues lists the values Linear's API actually accepts
// for ProjectRelationCreateInput/ProjectRelationUpdateInput's `type` field.
// The field is typed as a plain String in schema.graphql (not a GraphQL
// enum, unlike IssueRelationType), but the API still enforces a fixed set
// server-side: confirmed live via an INVALID_INPUT/isEnum validation error
// ("type must be one of the following values: dependency") when this CLI
// was first built against a guessed "blocks" value.
var projectRelationTypeValues = []string{"dependency"}

// parseProjectRelationType validates a --type value against the real,
// live-confirmed set of accepted values (case-insensitively) and returns
// the canonical string.
func parseProjectRelationType(t string) (string, error) {
	for _, v := range projectRelationTypeValues {
		if strings.EqualFold(v, t) {
			return v, nil
		}
	}
	return "", fmt.Errorf("Invalid relation type '%s'. Valid values: %s", t, strings.Join(projectRelationTypeValues, ", "))
}

// projectCmd represents the project command
var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage Linear projects",
	Long: `Manage Linear projects including listing, viewing, and creating projects.

Examples:
  lincli project list                      # List active projects
  lincli project list --include-completed  # List all projects including completed
  lincli project list --newer-than 1_month_ago  # List projects from last month
  lincli project get PROJECT-ID            # Get project details
  lincli project create                    # Create a new project`,
}

var projectListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List projects",
	Long:    `List all projects in your Linear workspace.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		// Get auth header
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		// Create API client
		client := api.NewClient(authHeader)

		// Get filters
		limit, _ := cmd.Flags().GetInt("limit")

		// Build typed filter
		filterTyped := buildProjectFilterTyped(cmd)

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
				output.Error(fmt.Sprintf("Invalid sort option: %s. Valid options are: linear, created, updated", sortBy), plaintext, jsonOut)
				os.Exit(1)
			}
		}

		// Convert limit to pointer
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		// Get projects
		resp, err := api.ListProjects(context.Background(), client, &filterTyped, limitPtr, nil, orderByEnum)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list projects: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		// Handle output
		if jsonOut {
			output.JSON(resp.Projects.Nodes)
			return
		} else if plaintext {
			fmt.Println("# Projects")
			for _, node := range resp.Projects.Nodes {
				f := node.ProjectListFields
				fmt.Printf("## %s\n", f.Name)
				fmt.Printf("- **ID**: %s\n", f.Id)
				fmt.Printf("- **State**: %s\n", f.State)
				fmt.Printf("- **Progress**: %.0f%%\n", f.Progress*100)
				if f.Lead != nil {
					fmt.Printf("- **Lead**: %s\n", f.Lead.Name)
				} else {
					fmt.Printf("- **Lead**: Unassigned\n")
				}
				if f.Teams != nil && len(f.Teams.Nodes) > 0 {
					teams := ""
					for i, team := range f.Teams.Nodes {
						if i > 0 {
							teams += ", "
						}
						teams += team.Key
					}
					fmt.Printf("- **Teams**: %s\n", teams)
				}
				if f.StartDate != nil {
					fmt.Printf("- **Start Date**: %s\n", *f.StartDate)
				}
				if f.TargetDate != nil {
					fmt.Printf("- **Target Date**: %s\n", *f.TargetDate)
				}
				fmt.Printf("- **Created**: %s\n", f.CreatedAt.Format("2006-01-02"))
				fmt.Printf("- **Updated**: %s\n", f.UpdatedAt.Format("2006-01-02"))
				fmt.Printf("- **URL**: %s\n", constructProjectURL(f.Id, f.Url))
				if f.Description != "" {
					fmt.Printf("- **Description**: %s\n", f.Description)
				}
				fmt.Println()
			}
			fmt.Printf("\nTotal: %d projects\n", len(resp.Projects.Nodes))
			return
		} else {
			// Table output
			headers := []string{"Name", "State", "Lead", "Teams", "Created", "Updated", "URL"}
			rows := [][]string{}

			for _, node := range resp.Projects.Nodes {
				f := node.ProjectListFields

				lead := color.New(color.FgYellow).Sprint("Unassigned")
				if f.Lead != nil {
					lead = f.Lead.Name
				}

				teams := ""
				if f.Teams != nil && len(f.Teams.Nodes) > 0 {
					for i, team := range f.Teams.Nodes {
						if i > 0 {
							teams += ", "
						}
						teams += team.Key
					}
				}

				stateColor := color.New(color.FgGreen)
				switch f.State {
				case "planned":
					stateColor = color.New(color.FgCyan)
				case "started":
					stateColor = color.New(color.FgBlue)
				case "paused":
					stateColor = color.New(color.FgYellow)
				case "completed":
					stateColor = color.New(color.FgGreen)
				case "canceled":
					stateColor = color.New(color.FgRed)
				}

				rows = append(rows, []string{
					truncateString(f.Name, 25),
					stateColor.Sprint(f.State),
					lead,
					teams,
					f.CreatedAt.Format("2006-01-02"),
					f.UpdatedAt.Format("2006-01-02"),
					constructProjectURL(f.Id, f.Url),
				})
			}

			output.Table(output.TableData{
				Headers: headers,
				Rows:    rows,
			}, plaintext, jsonOut)

			if !plaintext && !jsonOut {
				fmt.Printf("\n%s %d projects\n",
					color.New(color.FgGreen).Sprint("✓"),
					len(resp.Projects.Nodes))
			}
		}
	},
}

var projectGetCmd = &cobra.Command{
	Use:     "get PROJECT-ID",
	Aliases: []string{"show"},
	Short:   "Get project details",
	Long:    `Get detailed information about a specific project.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		projectID := args[0]

		// Get auth header
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		// Create API client
		client := api.NewClient(authHeader)

		// Get project details
		resp, err := api.GetProject(context.Background(), client, projectID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get project: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		project := resp.Project

		// Handle output
		if jsonOut {
			output.JSON(project.ProjectDetailFields)
		} else if plaintext {
			f := project.ProjectDetailFields
			fmt.Printf("# %s\n\n", f.Name)

			if f.Description != "" {
				fmt.Printf("## Description\n%s\n\n", f.Description)
			}

			if f.Content != nil && *f.Content != "" {
				fmt.Printf("## Content\n%s\n\n", *f.Content)
			}

			fmt.Printf("## Core Details\n")
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Slug ID**: %s\n", f.SlugId)
			fmt.Printf("- **State**: %s\n", f.State)
			fmt.Printf("- **Progress**: %.0f%%\n", f.Progress*100)
			if f.Health != nil {
				fmt.Printf("- **Health**: %s\n", *f.Health)
			}
			fmt.Printf("- **Scope**: %.0f\n", f.Scope)
			if f.Icon != nil && *f.Icon != "" {
				fmt.Printf("- **Icon**: %s\n", *f.Icon)
			}
			fmt.Printf("- **Color**: %s\n", f.Color)

			fmt.Printf("\n## Timeline\n")
			if f.StartDate != nil {
				fmt.Printf("- **Start Date**: %s\n", *f.StartDate)
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
			if f.Lead != nil {
				fmt.Printf("- **Lead**: %s (%s)\n", f.Lead.Name, f.Lead.Email)
				if f.Lead.DisplayName != "" && f.Lead.DisplayName != f.Lead.Name {
					fmt.Printf("  - Display Name: %s\n", f.Lead.DisplayName)
				}
			} else {
				fmt.Printf("- **Lead**: Unassigned\n")
			}
			if f.Creator != nil {
				fmt.Printf("- **Creator**: %s (%s)\n", f.Creator.Name, f.Creator.Email)
			}

			fmt.Printf("\n## Slack Integration\n")
			fmt.Printf("- **Slack New Issue**: %v\n", f.SlackNewIssue)
			fmt.Printf("- **Slack Issue Comments**: %v\n", f.SlackIssueComments)
			fmt.Printf("- **Slack Issue Statuses**: %v\n", f.SlackIssueStatuses)

			if f.ConvertedFromIssue != nil {
				fmt.Printf("\n## Origin\n")
				fmt.Printf("- **Converted from Issue**: %s - %s\n", f.ConvertedFromIssue.Identifier, f.ConvertedFromIssue.Title)
			}

			if f.LastAppliedTemplate != nil {
				fmt.Printf("\n## Template\n")
				fmt.Printf("- **Last Applied**: %s\n", f.LastAppliedTemplate.Name)
				if f.LastAppliedTemplate.Description != nil && *f.LastAppliedTemplate.Description != "" {
					fmt.Printf("  - Description: %s\n", *f.LastAppliedTemplate.Description)
				}
			}

			// Teams
			if f.Teams != nil && len(f.Teams.Nodes) > 0 {
				fmt.Printf("\n## Teams\n")
				for _, team := range f.Teams.Nodes {
					fmt.Printf("- **%s** (%s)\n", team.Name, team.Key)
					if team.Description != nil && *team.Description != "" {
						fmt.Printf("  - Description: %s\n", *team.Description)
					}
					fmt.Printf("  - Cycles Enabled: %v\n", team.CyclesEnabled)
				}
			}

			fmt.Printf("\n## URL\n")
			fmt.Printf("- %s\n", constructProjectURL(f.Id, f.Url))

			// Show members if available
			if f.Members != nil && len(f.Members.Nodes) > 0 {
				fmt.Printf("\n## Members\n")
				for _, member := range f.Members.Nodes {
					fmt.Printf("- %s (%s)", member.Name, member.Email)
					if member.DisplayName != "" && member.DisplayName != member.Name {
						fmt.Printf(" - %s", member.DisplayName)
					}
					if member.Admin {
						fmt.Printf(" [Admin]")
					}
					if !member.Active {
						fmt.Printf(" [Inactive]")
					}
					fmt.Println()
				}
			}

			// Project Updates
			if f.ProjectUpdates != nil && len(f.ProjectUpdates.Nodes) > 0 {
				fmt.Printf("\n## Recent Project Updates\n")
				for _, update := range f.ProjectUpdates.Nodes {
					fmt.Printf("\n### %s by %s\n", update.CreatedAt.Format("2006-01-02 15:04"), update.User.Name)
					if update.EditedAt != nil {
						fmt.Printf("*(edited %s)*\n", update.EditedAt.Format("2006-01-02 15:04"))
					}
					fmt.Printf("- **Health**: %s\n", update.Health)
					fmt.Printf("\n%s\n", update.Body)
				}
			}

			// Documents
			if f.Documents != nil && len(f.Documents.Nodes) > 0 {
				fmt.Printf("\n## Documents\n")
				for _, doc := range f.Documents.Nodes {
					fmt.Printf("\n### %s\n", doc.Title)
					if doc.Icon != nil && *doc.Icon != "" {
						fmt.Printf("- **Icon**: %s\n", *doc.Icon)
					}
					fmt.Printf("- **Color**: %s\n", derefStr(doc.Color))
					fmt.Printf("- **Created**: %s by %s\n", doc.CreatedAt.Format("2006-01-02"), doc.Creator.Name)
					if doc.UpdatedBy != nil {
						fmt.Printf("- **Updated**: %s by %s\n", doc.UpdatedAt.Format("2006-01-02"), doc.UpdatedBy.Name)
					}
					fmt.Printf("\n%s\n", derefStr(doc.Content))
				}
			}

			// Labels
			if f.Labels != nil && len(f.Labels.Nodes) > 0 {
				fmt.Printf("\n## Labels\n")
				for _, l := range f.Labels.Nodes {
					fmt.Printf("- %s\n", l.Name)
				}
			}

			// Relations
			hasRelations := f.Relations != nil && len(f.Relations.Nodes) > 0
			hasInverseRelations := f.InverseRelations != nil && len(f.InverseRelations.Nodes) > 0
			if hasRelations || hasInverseRelations {
				fmt.Printf("\n## Relations\n")
				if hasRelations {
					for _, r := range f.Relations.Nodes {
						fmt.Printf("- %s: %s\n", r.Type, r.RelatedProject.Name)
					}
				}
				if hasInverseRelations {
					for _, r := range f.InverseRelations.Nodes {
						fmt.Printf("- %s (inverse): %s\n", r.Type, r.Project.Name)
					}
				}
			}

			// Show recent issues
			if f.Issues != nil && len(f.Issues.Nodes) > 0 {
				fmt.Printf("\n## Issues (%d total)\n", len(f.Issues.Nodes))
				for _, issue := range f.Issues.Nodes {
					stateStr := ""
					if issue.State != nil {
						switch issue.State.Type {
						case "completed":
							stateStr = "[x]"
						case "started":
							stateStr = "[~]"
						case "canceled":
							stateStr = "[-]"
						default:
							stateStr = "[ ]"
						}
					} else {
						stateStr = "[ ]"
					}

					assignee := "Unassigned"
					if issue.Assignee != nil {
						assignee = issue.Assignee.Name
					}

					fmt.Printf("\n### %s %s (#%.0f)\n", stateStr, issue.Identifier, issue.Number)
					fmt.Printf("**%s**\n", issue.Title)
					fmt.Printf("- Assignee: %s\n", assignee)
					fmt.Printf("- Priority: %s\n", priorityToString(int(issue.Priority)))
					if issue.Estimate != nil {
						fmt.Printf("- Estimate: %.1f\n", *issue.Estimate)
					}
					if issue.State != nil {
						fmt.Printf("- State: %s\n", issue.State.Name)
					}
					if issue.Labels != nil && len(issue.Labels.Nodes) > 0 {
						labels := []string{}
						for _, label := range issue.Labels.Nodes {
							labels = append(labels, label.Name)
						}
						fmt.Printf("- Labels: %s\n", strings.Join(labels, ", "))
					}
					fmt.Printf("- Updated: %s\n", issue.UpdatedAt.Format("2006-01-02 15:04"))
					if issue.Description != nil && *issue.Description != "" {
						// Show first 3 lines of description
						lines := strings.Split(*issue.Description, "\n")
						preview := ""
						for i, line := range lines {
							if i >= 3 {
								preview += "\n  ..."
								break
							}
							if i > 0 {
								preview += "\n  "
							}
							preview += line
						}
						fmt.Printf("- Description: %s\n", preview)
					}
				}
			}
		} else {
			// Formatted output
			f := project.ProjectDetailFields
			fmt.Println()
			fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("📁 Project:"), f.Name)
			fmt.Println(strings.Repeat("─", 50))

			fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("ID:"), f.Id)

			if f.Description != "" {
				fmt.Printf("\n%s\n%s\n", color.New(color.Bold).Sprint("Description:"), f.Description)
			}

			stateColor := color.New(color.FgGreen)
			switch f.State {
			case "planned":
				stateColor = color.New(color.FgCyan)
			case "started":
				stateColor = color.New(color.FgBlue)
			case "paused":
				stateColor = color.New(color.FgYellow)
			case "completed":
				stateColor = color.New(color.FgGreen)
			case "canceled":
				stateColor = color.New(color.FgRed)
			}
			fmt.Printf("\n%s %s\n", color.New(color.Bold).Sprint("State:"), stateColor.Sprint(f.State))

			progressColor := color.New(color.FgRed)
			if f.Progress >= 0.75 {
				progressColor = color.New(color.FgGreen)
			} else if f.Progress >= 0.5 {
				progressColor = color.New(color.FgYellow)
			}
			fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Progress:"), progressColor.Sprintf("%.0f%%", f.Progress*100))

			if f.StartDate != nil || f.TargetDate != nil {
				fmt.Println()
				if f.StartDate != nil {
					fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Start Date:"), *f.StartDate)
				}
				if f.TargetDate != nil {
					fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Target Date:"), *f.TargetDate)
				}
			}

			if f.Lead != nil {
				fmt.Printf("\n%s %s (%s)\n",
					color.New(color.Bold).Sprint("Lead:"),
					f.Lead.Name,
					color.New(color.FgCyan).Sprint(f.Lead.Email))
			}

			if f.Teams != nil && len(f.Teams.Nodes) > 0 {
				fmt.Printf("\n%s\n", color.New(color.Bold).Sprint("Teams:"))
				for _, team := range f.Teams.Nodes {
					fmt.Printf("  • %s - %s\n",
						color.New(color.FgCyan).Sprint(team.Key),
						team.Name)
				}
			}

			// Show members if available
			if f.Members != nil && len(f.Members.Nodes) > 0 {
				fmt.Printf("\n%s\n", color.New(color.Bold).Sprint("Members:"))
				for _, member := range f.Members.Nodes {
					fmt.Printf("  • %s (%s)\n",
						member.Name,
						color.New(color.FgCyan).Sprint(member.Email))
				}
			}

			// Recent project updates (status-update posts, distinct from the
			// project record itself)
			if f.ProjectUpdates != nil && len(f.ProjectUpdates.Nodes) > 0 {
				fmt.Printf("\n%s\n", color.New(color.Bold).Sprint("Recent Updates:"))
				for _, update := range f.ProjectUpdates.Nodes {
					author := "Unknown"
					if update.User != nil {
						author = update.User.Name
					}
					fmt.Printf("  • [%s] %s by %s\n",
						string(update.Health),
						update.CreatedAt.Format("2006-01-02 15:04"),
						author)
				}
			}

			// Labels
			if f.Labels != nil && len(f.Labels.Nodes) > 0 {
				fmt.Printf("\n%s\n", color.New(color.Bold).Sprint("Labels:"))
				for _, l := range f.Labels.Nodes {
					fmt.Printf("  • %s\n", l.Name)
				}
			}

			// Relations
			hasRelationsRich := f.Relations != nil && len(f.Relations.Nodes) > 0
			hasInverseRelationsRich := f.InverseRelations != nil && len(f.InverseRelations.Nodes) > 0
			if hasRelationsRich || hasInverseRelationsRich {
				fmt.Printf("\n%s\n", color.New(color.Bold).Sprint("Relations:"))
				if hasRelationsRich {
					for _, r := range f.Relations.Nodes {
						fmt.Printf("  • %s: %s\n", r.Type, r.RelatedProject.Name)
					}
				}
				if hasInverseRelationsRich {
					for _, r := range f.InverseRelations.Nodes {
						fmt.Printf("  • %s (inverse): %s\n", r.Type, r.Project.Name)
					}
				}
			}

			// Show sample issues if available
			if f.Issues != nil && len(f.Issues.Nodes) > 0 {
				fmt.Printf("\n%s\n", color.New(color.Bold).Sprint("Recent Issues:"))
				for i, issue := range f.Issues.Nodes {
					if i >= 5 {
						break // Show only first 5
					}
					stateIcon := "○"
					if issue.State != nil {
						switch issue.State.Type {
						case "completed":
							stateIcon = color.New(color.FgGreen).Sprint("✓")
						case "started":
							stateIcon = color.New(color.FgBlue).Sprint("◐")
						case "canceled":
							stateIcon = color.New(color.FgRed).Sprint("✗")
						}
					}
					assignee := "Unassigned"
					if issue.Assignee != nil {
						assignee = issue.Assignee.Name
					}
					fmt.Printf("  %s %s %s (%s)\n",
						stateIcon,
						color.New(color.FgCyan).Sprint(issue.Identifier),
						issue.Title,
						color.New(color.FgWhite, color.Faint).Sprint(assignee))
				}
			}

			// Show timestamps
			fmt.Printf("\n%s\n", color.New(color.Bold).Sprint("Timeline:"))
			fmt.Printf("  Created: %s\n", f.CreatedAt.Format("2006-01-02"))
			fmt.Printf("  Updated: %s\n", f.UpdatedAt.Format("2006-01-02"))
			if f.CompletedAt != nil {
				fmt.Printf("  Completed: %s\n", f.CompletedAt.Format("2006-01-02"))
			}
			if f.CanceledAt != nil {
				fmt.Printf("  Canceled: %s\n", f.CanceledAt.Format("2006-01-02"))
			}

			// Show URL
			if f.Url != "" {
				fmt.Printf("\n%s %s\n",
					color.New(color.Bold).Sprint("URL:"),
					color.New(color.FgBlue, color.Underline).Sprint(constructProjectURL(f.Id, f.Url)))
			}

			fmt.Println()
		}
	},
}

var projectCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a new project",
	Long:    `Create a new project in Linear.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			output.Error("Name is required (--name)", plaintext, jsonOut)
			os.Exit(1)
		}

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
			output.Error(fmt.Sprintf("Failed to find team '%s': %v", team, err), plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.ProjectCreateInput{
			Name:    name,
			TeamIds: []string{teamID},
		}

		if description, _ := cmd.Flags().GetString("description"); description != "" {
			input.Description = &description
		}

		if lead, _ := cmd.Flags().GetString("lead"); lead != "" {
			leadID, err := resolveUser(ctx, client, cache, lead)
			if err != nil {
				output.Error(fmt.Sprintf("Failed to find lead '%s': %v", lead, err), plaintext, jsonOut)
				os.Exit(1)
			}
			input.LeadId = &leadID
		}

		if startDate, _ := cmd.Flags().GetString("start-date"); startDate != "" {
			input.StartDate = &startDate
		}

		if targetDate, _ := cmd.Flags().GetString("target-date"); targetDate != "" {
			input.TargetDate = &targetDate
		}

		if status, _ := cmd.Flags().GetString("status"); status != "" {
			statusID, err := resolveProjectStatus(ctx, client, cache, status)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.StatusId = &statusID
		}

		// --template applies a project template's pre-filled attributes. It is
		// resolved by name within the "project" template type (or passed as an ID).
		if cmd.Flags().Changed("template") {
			template, _ := cmd.Flags().GetString("template")
			templateID, err := resolveTemplate(ctx, client, cache, "project", template)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.TemplateId = &templateID
		}

		createResp, err := api.CreateProject(ctx, client, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create project: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !createResp.ProjectCreate.Success || createResp.ProjectCreate.Project == nil {
			output.Error("Failed to create project", plaintext, jsonOut)
			os.Exit(1)
		}

		project := createResp.ProjectCreate.Project

		if jsonOut {
			output.JSON(project)
		} else if plaintext {
			fmt.Printf("Created project: %s\n", project.ProjectListFields.Name)
		} else {
			fmt.Printf("%s Created project: %s\n",
				color.New(color.FgGreen).Sprint("✓"),
				color.New(color.FgCyan, color.Bold).Sprint(project.ProjectListFields.Name))
		}
	},
}

var projectUpdateCmd = &cobra.Command{
	Use:   "update <project-id-or-name>",
	Short: "Update a project",
	Long: `Update a project's record: name, description, lead, start/target dates, or status.

This does not change a project's state (planned/started/paused/completed/canceled)
directly - state is derived from the assigned status - and does not post a
project update (a separate, later CLI pass).

Examples:
  lincli project update ID --status "In Progress"
  lincli project update "My Project" --lead jane@example.com
  lincli project update ID --target-date 2025-12-31
  lincli project update ID --lead unassigned`,
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.ProjectUpdateInput{}

		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
		}

		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
		}

		if cmd.Flags().Changed("lead") {
			lead, _ := cmd.Flags().GetString("lead")
			switch strings.ToLower(lead) {
			case "unassigned", "":
				// Nested input fields need an explicit JSON null to clear -
				// a plain nil pointer gets stripped out of the request
				// entirely (see stripNulls in pkg/api/client.go), which
				// Linear treats as "leave unchanged" rather than "clear".
				nullVal := api.NullSentinel
				input.LeadId = &nullVal
			default:
				leadID, err := resolveUser(ctx, client, cache, lead)
				if err != nil {
					output.Error(fmt.Sprintf("Failed to find lead '%s': %v", lead, err), plaintext, jsonOut)
					os.Exit(1)
				}
				input.LeadId = &leadID
			}
		}

		if cmd.Flags().Changed("start-date") {
			startDate, _ := cmd.Flags().GetString("start-date")
			if startDate == "" {
				nullVal := api.NullSentinel
				input.StartDate = &nullVal
			} else {
				input.StartDate = &startDate
			}
		}

		if cmd.Flags().Changed("target-date") {
			targetDate, _ := cmd.Flags().GetString("target-date")
			if targetDate == "" {
				nullVal := api.NullSentinel
				input.TargetDate = &nullVal
			} else {
				input.TargetDate = &targetDate
			}
		}

		if cmd.Flags().Changed("status") {
			status, _ := cmd.Flags().GetString("status")
			statusID, err := resolveProjectStatus(ctx, client, cache, status)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.StatusId = &statusID
		}

		hasUpdates := input.Name != nil ||
			input.Description != nil ||
			cmd.Flags().Changed("lead") ||
			cmd.Flags().Changed("start-date") ||
			cmd.Flags().Changed("target-date") ||
			input.StatusId != nil

		if !hasUpdates {
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		updateResp, err := api.UpdateProject(ctx, client, projectID, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update project: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !updateResp.ProjectUpdate.Success || updateResp.ProjectUpdate.Project == nil {
			output.Error("Failed to update project", plaintext, jsonOut)
			os.Exit(1)
		}

		updated := updateResp.ProjectUpdate.Project

		if jsonOut {
			output.JSON(updated)
		} else if plaintext {
			fmt.Printf("Updated project %s\n", updated.ProjectListFields.Name)
		} else {
			output.Success(fmt.Sprintf("Updated project %s", updated.ProjectListFields.Name), plaintext, jsonOut)
		}
	},
}

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <project-id-or-name>",
	Short: "Delete a project",
	Long:  `Delete (trash) a project. This action executes immediately with no confirmation prompt. The project can be restored later with 'project unarchive'.`,
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectDelete(ctx, client, projectID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete project: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectDelete.Success {
			output.Error("Failed to delete project", plaintext, jsonOut)
			os.Exit(1)
		}

		name := projectID
		if resp.ProjectDelete.Entity != nil {
			name = resp.ProjectDelete.Entity.Name
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": projectID})
		} else if plaintext {
			fmt.Printf("Deleted project %s\n", name)
		} else {
			output.Success(fmt.Sprintf("Deleted project %s", name), plaintext, jsonOut)
		}
	},
}

var projectUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <project-id-or-name>",
	Short: "Unarchive a project",
	Long:  `Unarchive (restore) a previously deleted/archived project. This action executes immediately with no confirmation prompt.`,
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectUnarchive(ctx, client, projectID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to unarchive project: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectUnarchive.Success {
			output.Error("Failed to unarchive project", plaintext, jsonOut)
			os.Exit(1)
		}

		name := projectID
		if resp.ProjectUnarchive.Entity != nil {
			name = resp.ProjectUnarchive.Entity.Name
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": projectID})
		} else if plaintext {
			fmt.Printf("Unarchived project %s\n", name)
		} else {
			output.Success(fmt.Sprintf("Unarchived project %s", name), plaintext, jsonOut)
		}
	},
}

// --- Project members ---

// getProjectMemberIDs fetches a project's current member IDs and name, used
// by 'project member add'/'remove' to build the full memberIds list expected
// by ProjectUpdateInput (Linear has no dedicated add/remove-single-member
// mutation - membership is always set as a complete list).
func getProjectMemberIDs(ctx context.Context, client graphql.Client, projectID string) ([]string, string, error) {
	resp, err := api.GetProject(ctx, client, projectID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get project: %w", err)
	}
	if resp.Project == nil {
		return nil, "", fmt.Errorf("project not found")
	}

	f := resp.Project.ProjectDetailFields
	ids := []string{}
	if f.Members != nil {
		for _, m := range f.Members.Nodes {
			ids = append(ids, m.Id)
		}
	}
	return ids, f.Name, nil
}

var projectMembersCmd = &cobra.Command{
	Use:   "members <project-id-or-name>",
	Short: "List a project's members",
	Long:  `List the members associated with a project.`,
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.GetProject(ctx, client, projectID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get project: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.Project == nil {
			output.Error("Project not found", plaintext, jsonOut)
			os.Exit(1)
		}

		f := resp.Project.ProjectDetailFields
		members := []*api.ProjectDetailFieldsMembersUserConnectionNodesUser{}
		if f.Members != nil {
			members = f.Members.Nodes
		}

		if len(members) == 0 {
			output.Info(fmt.Sprintf("No members found for project %s", f.Name), plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(members)
			return
		}

		if plaintext {
			fmt.Printf("# Members of %s\n", f.Name)
			for _, m := range members {
				fmt.Printf("- %s (%s)", m.Name, m.Email)
				if m.Admin {
					fmt.Printf(" [Admin]")
				}
				if !m.Active {
					fmt.Printf(" [Inactive]")
				}
				fmt.Println()
			}
			return
		}

		headers := []string{"Name", "Email", "Admin", "Active"}
		rows := make([][]string, len(members))
		for i, m := range members {
			rows[i] = []string{m.Name, m.Email, fmt.Sprintf("%v", m.Admin), fmt.Sprintf("%v", m.Active)}
		}
		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)
	},
}

var projectMemberCmd = &cobra.Command{
	Use:   "member",
	Short: "Manage a project's members",
	Long: `Add and remove members on a project.

Linear has no dedicated add/remove-single-member mutation: membership is
always set as a complete memberIds list on ProjectUpdateInput, so these
commands read the project's current members, add/remove the resolved user,
and write the full list back.

Examples:
  lincli project member add "My Project" jane@example.com
  lincli project member remove "My Project" jane@example.com`,
}

var projectMemberAddCmd = &cobra.Command{
	Use:   "add <project-id-or-name> <user>",
	Short: "Add a member to a project",
	Args:  cobra.ExactArgs(2),
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}
		userID, err := resolveUser(ctx, client, cache, args[1])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to find user '%s': %v", args[1], err), plaintext, jsonOut)
			os.Exit(1)
		}

		memberIDs, projectName, err := getProjectMemberIDs(ctx, client, projectID)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		for _, id := range memberIDs {
			if id == userID {
				output.Error("User is already a member of this project", plaintext, jsonOut)
				os.Exit(1)
			}
		}
		memberIDs = append(memberIDs, userID)

		input := api.ProjectUpdateInput{MemberIds: memberIDs}
		resp, err := api.UpdateProject(ctx, client, projectID, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to add member to project: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectUpdate.Success {
			output.Error("Failed to add member to project", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.ProjectUpdate.Project)
		} else if plaintext {
			fmt.Printf("Added member to project %s\n", projectName)
		} else {
			output.Success(fmt.Sprintf("Added member to project %s", projectName), plaintext, jsonOut)
		}
	},
}

var projectMemberRemoveCmd = &cobra.Command{
	Use:     "remove <project-id-or-name> <user>",
	Aliases: []string{"rm"},
	Short:   "Remove a member from a project",
	Long:    `Remove a member from a project. This action executes immediately with no confirmation prompt.`,
	Args:    cobra.ExactArgs(2),
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}
		userID, err := resolveUser(ctx, client, cache, args[1])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to find user '%s': %v", args[1], err), plaintext, jsonOut)
			os.Exit(1)
		}

		memberIDs, projectName, err := getProjectMemberIDs(ctx, client, projectID)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		found := false
		filtered := make([]string, 0, len(memberIDs))
		for _, id := range memberIDs {
			if id == userID {
				found = true
				continue
			}
			filtered = append(filtered, id)
		}
		if !found {
			output.Error("User is not a member of this project", plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.ProjectUpdateInput{MemberIds: filtered}
		resp, err := api.UpdateProject(ctx, client, projectID, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to remove member from project: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectUpdate.Success {
			output.Error("Failed to remove member from project", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.ProjectUpdate.Project)
		} else if plaintext {
			fmt.Printf("Removed member from project %s\n", projectName)
		} else {
			output.Success(fmt.Sprintf("Removed member from project %s", projectName), plaintext, jsonOut)
		}
	},
}

// --- Project milestones ---

// resolveMilestoneArg resolves a milestone argument that may be either a raw
// ID (the common case - milestone IDs come from 'project milestone list'
// output) or a name. isUUID short-circuits the ID case without touching
// --project at all; a name additionally requires --project so resolveMilestone
// has a project to scope the name lookup to.
func resolveMilestoneArg(ctx context.Context, client graphql.Client, cache *ResolverCache, cmd *cobra.Command, arg string) (string, error) {
	if isUUID(arg) {
		return arg, nil
	}

	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		return "", fmt.Errorf("'%s' is not a milestone ID; pass --project to resolve it by name", arg)
	}
	projectID, err := resolveProject(ctx, client, cache, project)
	if err != nil {
		return "", err
	}
	return resolveMilestone(ctx, client, cache, projectID, arg)
}

var projectMilestoneCmd = &cobra.Command{
	Use:   "milestone",
	Short: "Manage a project's milestones",
	Long: `List, view, create, update, move, and delete milestones within a project.

'get'/'update'/'delete' accept a milestone ID directly (as shown by 'list'),
or a name scoped with --project.

Examples:
  lincli project milestone list "My Project"
  lincli project milestone create "My Project" --name "Beta" --target-date 2025-06-01
  lincli project milestone get MILESTONE-ID
  lincli project milestone update MILESTONE-ID --name "Beta v2"
  lincli project milestone move MILESTONE-ID --project "Other Project"
  lincli project milestone delete MILESTONE-ID`,
}

var projectMilestoneListCmd = &cobra.Command{
	Use:     "list <project-id-or-name>",
	Aliases: []string{"ls"},
	Short:   "List a project's milestones",
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		resp, err := api.ListProjectMilestones(ctx, client, projectID, limitPtr)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list milestones: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.Project == nil || resp.Project.ProjectMilestones == nil {
			output.Error("Project not found", plaintext, jsonOut)
			os.Exit(1)
		}

		milestones := resp.Project.ProjectMilestones.Nodes

		if len(milestones) == 0 {
			output.Info("No milestones found for this project", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(milestones)
			return
		}

		if plaintext {
			fmt.Println("# Project Milestones")
			for _, m := range milestones {
				f := m.ProjectMilestoneFields
				fmt.Printf("## %s\n", f.Name)
				fmt.Printf("- **ID**: %s\n", f.Id)
				fmt.Printf("- **Status**: %s\n", f.Status)
				fmt.Printf("- **Progress**: %.0f%%\n", f.Progress*100)
				if f.TargetDate != nil {
					fmt.Printf("- **Target Date**: %s\n", *f.TargetDate)
				}
				if f.Description != nil && *f.Description != "" {
					fmt.Printf("- **Description**: %s\n", *f.Description)
				}
				fmt.Println()
			}
			return
		}

		headers := []string{"Name", "Status", "Progress", "Target Date"}
		rows := make([][]string, len(milestones))
		for i, m := range milestones {
			f := m.ProjectMilestoneFields
			targetDate := ""
			if f.TargetDate != nil {
				targetDate = *f.TargetDate
			}
			rows[i] = []string{
				f.Name,
				string(f.Status),
				fmt.Sprintf("%.0f%%", f.Progress*100),
				targetDate,
			}
		}
		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)
	},
}

var projectMilestoneGetCmd = &cobra.Command{
	Use:     "get <milestone-id-or-name>",
	Aliases: []string{"show"},
	Short:   "Get milestone details",
	Long:    `Get detailed information about a project milestone. Accepts a milestone ID directly, or a name scoped with --project.`,
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

		milestoneID, err := resolveMilestoneArg(ctx, client, cache, cmd, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.GetProjectMilestone(ctx, client, milestoneID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get milestone: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.ProjectMilestone == nil {
			output.Error("Milestone not found", plaintext, jsonOut)
			os.Exit(1)
		}
		f := resp.ProjectMilestone.ProjectMilestoneFields

		if jsonOut {
			output.JSON(resp.ProjectMilestone)
			return
		}

		if plaintext {
			fmt.Printf("# %s\n\n", f.Name)
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Status**: %s\n", f.Status)
			fmt.Printf("- **Progress**: %.0f%%\n", f.Progress*100)
			if f.TargetDate != nil {
				fmt.Printf("- **Target Date**: %s\n", *f.TargetDate)
			}
			if resp.ProjectMilestone.Project != nil {
				fmt.Printf("- **Project**: %s\n", resp.ProjectMilestone.Project.Name)
			}
			if f.Description != nil && *f.Description != "" {
				fmt.Printf("\n## Description\n%s\n", *f.Description)
			}
			return
		}

		fmt.Println()
		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Milestone:"), f.Name)
		fmt.Println(strings.Repeat("─", 50))
		fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("ID:"), f.Id)
		fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Status:"), string(f.Status))
		fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Progress:"), fmt.Sprintf("%.0f%%", f.Progress*100))
		if f.TargetDate != nil {
			fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Target Date:"), *f.TargetDate)
		}
		if resp.ProjectMilestone.Project != nil {
			fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Project:"), resp.ProjectMilestone.Project.Name)
		}
		if f.Description != nil && *f.Description != "" {
			fmt.Printf("\n%s\n%s\n", color.New(color.Bold).Sprint("Description:"), *f.Description)
		}
		fmt.Println()
	},
}

var projectMilestoneCreateCmd = &cobra.Command{
	Use:     "create <project-id-or-name>",
	Aliases: []string{"new"},
	Short:   "Create a new milestone within a project",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			output.Error("Name is required (--name)", plaintext, jsonOut)
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.ProjectMilestoneCreateInput{
			Name:      name,
			ProjectId: projectID,
		}

		if description, _ := cmd.Flags().GetString("description"); description != "" {
			input.Description = &description
		}
		if targetDate, _ := cmd.Flags().GetString("target-date"); targetDate != "" {
			input.TargetDate = &targetDate
		}
		if cmd.Flags().Changed("sort-order") {
			sortOrder, _ := cmd.Flags().GetFloat64("sort-order")
			input.SortOrder = &sortOrder
		}

		resp, err := api.ProjectMilestoneCreate(ctx, client, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create milestone: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectMilestoneCreate.Success || resp.ProjectMilestoneCreate.ProjectMilestone == nil {
			output.Error("Failed to create milestone", plaintext, jsonOut)
			os.Exit(1)
		}

		milestone := resp.ProjectMilestoneCreate.ProjectMilestone

		if jsonOut {
			output.JSON(milestone)
		} else if plaintext {
			fmt.Printf("Created milestone: %s\n", milestone.ProjectMilestoneFields.Name)
		} else {
			output.Success(fmt.Sprintf("Created milestone %s", milestone.ProjectMilestoneFields.Name), plaintext, jsonOut)
		}
	},
}

var projectMilestoneUpdateCmd = &cobra.Command{
	Use:   "update <milestone-id-or-name>",
	Short: "Update a project milestone",
	Long:  `Update a milestone's name, description, target date, or sort order. Accepts a milestone ID directly, or a name scoped with --project.`,
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

		milestoneID, err := resolveMilestoneArg(ctx, client, cache, cmd, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.ProjectMilestoneUpdateInput{}

		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
		}
		if cmd.Flags().Changed("target-date") {
			targetDate, _ := cmd.Flags().GetString("target-date")
			if targetDate == "" {
				nullVal := api.NullSentinel
				input.TargetDate = &nullVal
			} else {
				input.TargetDate = &targetDate
			}
		}
		if cmd.Flags().Changed("sort-order") {
			sortOrder, _ := cmd.Flags().GetFloat64("sort-order")
			input.SortOrder = &sortOrder
		}

		hasUpdates := input.Name != nil || input.Description != nil || cmd.Flags().Changed("target-date") || input.SortOrder != nil
		if !hasUpdates {
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectMilestoneUpdate(ctx, client, milestoneID, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update milestone: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectMilestoneUpdate.Success || resp.ProjectMilestoneUpdate.ProjectMilestone == nil {
			output.Error("Failed to update milestone", plaintext, jsonOut)
			os.Exit(1)
		}

		updated := resp.ProjectMilestoneUpdate.ProjectMilestone

		if jsonOut {
			output.JSON(updated)
		} else if plaintext {
			fmt.Printf("Updated milestone %s\n", updated.ProjectMilestoneFields.Name)
		} else {
			output.Success(fmt.Sprintf("Updated milestone %s", updated.ProjectMilestoneFields.Name), plaintext, jsonOut)
		}
	},
}

var projectMilestoneMoveCmd = &cobra.Command{
	Use:   "move <milestone-id> --project <project-id-or-name>",
	Short: "Move a milestone to another project",
	Long: `Move a milestone to a different project, optionally setting its sort order
within the destination project afterward.

The milestone argument here must be an ID (--project already means the move
destination, so it can't also scope a name lookup for the milestone itself -
use 'project milestone list' to find the ID).

ProjectMilestoneMoveInput has no sortOrder field (verified against
schema.graphql); when --sort-order is given, this runs the move followed by
a separate update call to set the sort order.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			output.Error("Destination project is required (--project)", plaintext, jsonOut)
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

		milestoneID, err := resolveMilestone(ctx, client, cache, "", args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}
		projectID, err := resolveProject(ctx, client, cache, project)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.ProjectMilestoneMoveInput{ProjectId: projectID}
		resp, err := api.ProjectMilestoneMove(ctx, client, milestoneID, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to move milestone: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectMilestoneMove.Success || resp.ProjectMilestoneMove.ProjectMilestone == nil {
			output.Error("Failed to move milestone", plaintext, jsonOut)
			os.Exit(1)
		}

		moved := resp.ProjectMilestoneMove.ProjectMilestone
		projectName := ""
		if moved.Project != nil {
			projectName = moved.Project.Name
		}

		if cmd.Flags().Changed("sort-order") {
			sortOrder, _ := cmd.Flags().GetFloat64("sort-order")
			updateInput := api.ProjectMilestoneUpdateInput{SortOrder: &sortOrder}
			updateResp, err := api.ProjectMilestoneUpdate(ctx, client, milestoneID, &updateInput)
			if err != nil {
				output.Error(fmt.Sprintf("Milestone moved to %s, but failed to set sort order: %v", projectName, err), plaintext, jsonOut)
				os.Exit(1)
			}
			if !updateResp.ProjectMilestoneUpdate.Success || updateResp.ProjectMilestoneUpdate.ProjectMilestone == nil {
				output.Error(fmt.Sprintf("Milestone moved to %s, but failed to set sort order", projectName), plaintext, jsonOut)
				os.Exit(1)
			}

			updated := updateResp.ProjectMilestoneUpdate.ProjectMilestone
			if jsonOut {
				output.JSON(updated)
			} else if plaintext {
				fmt.Printf("Moved milestone %s to project %s and updated sort order\n", updated.ProjectMilestoneFields.Name, projectName)
			} else {
				output.Success(fmt.Sprintf("Moved milestone %s to project %s and updated sort order", updated.ProjectMilestoneFields.Name, projectName), plaintext, jsonOut)
			}
			return
		}

		if jsonOut {
			output.JSON(moved)
		} else if plaintext {
			fmt.Printf("Moved milestone %s to project %s\n", moved.ProjectMilestoneFields.Name, projectName)
		} else {
			output.Success(fmt.Sprintf("Moved milestone %s to project %s", moved.ProjectMilestoneFields.Name, projectName), plaintext, jsonOut)
		}
	},
}

var projectMilestoneDeleteCmd = &cobra.Command{
	Use:   "delete <milestone-id-or-name>",
	Short: "Delete a project milestone",
	Long:  `Delete a project milestone. This action executes immediately with no confirmation prompt. Accepts a milestone ID directly, or a name scoped with --project.`,
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

		milestoneID, err := resolveMilestoneArg(ctx, client, cache, cmd, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectMilestoneDelete(ctx, client, milestoneID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete milestone: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectMilestoneDelete.Success {
			output.Error("Failed to delete milestone", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.ProjectMilestoneDelete.EntityId})
		} else if plaintext {
			fmt.Printf("Deleted milestone %s\n", resp.ProjectMilestoneDelete.EntityId)
		} else {
			output.Success(fmt.Sprintf("Deleted milestone %s", resp.ProjectMilestoneDelete.EntityId), plaintext, jsonOut)
		}
	},
}

// --- Project status-update posts ---

var projectUpdatePostCmd = &cobra.Command{
	Use:   "update-post",
	Short: "Manage project status-update posts",
	Long: `List, create, edit, archive, and unarchive status updates posted to a
project.

Kept distinct from 'project update' (which edits the project record itself -
name, description, lead, dates, status, etc): these are point-in-time
status posts shown on the project's timeline, each capturing a health
snapshot (onTrack/atRisk/offTrack).

Examples:
  lincli project update-post list "My Project"
  lincli project update-post create "My Project" --body "Kicking off" --health onTrack
  lincli project update-post edit UPDATE-ID --body "Revised" --health atRisk
  lincli project update-post archive UPDATE-ID`,
}

var projectUpdatePostListCmd = &cobra.Command{
	Use:     "list <project-id-or-name>",
	Aliases: []string{"ls"},
	Short:   "List status updates posted to a project",
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		filter := &api.ProjectUpdateFilter{
			Project: &api.ProjectFilter{
				Id: &api.IDComparator{Eq: &projectID},
			},
		}

		resp, err := api.ListProjectUpdates(ctx, client, filter, limitPtr, nil, nil)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list project updates: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		updates := resp.ProjectUpdates.Nodes

		if len(updates) == 0 {
			output.Info("No status updates found for this project", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(updates)
			return
		}

		if plaintext {
			fmt.Println("# Project Status Updates")
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
			return
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
	},
}

var projectUpdatePostCreateCmd = &cobra.Command{
	Use:     "create <project-id-or-name>",
	Aliases: []string{"add", "new"},
	Short:   "Post a new status update to a project",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		body, _ := cmd.Flags().GetString("body")
		if body == "" {
			output.Error("Body is required (--body)", plaintext, jsonOut)
			os.Exit(1)
		}

		var health *api.ProjectUpdateHealthType
		if healthStr, _ := cmd.Flags().GetString("health"); healthStr != "" {
			h, err := parseProjectUpdateHealth(healthStr)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			health = &h
		}

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()
		cache := newResolverCache()

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.ProjectUpdateCreateInput{
			ProjectId: projectID,
			Body:      &body,
			Health:    health,
		}

		resp, err := api.ProjectUpdateCreate(ctx, client, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to post project update: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectUpdateCreate.Success {
			output.Error("Failed to post project update", plaintext, jsonOut)
			os.Exit(1)
		}

		update := resp.ProjectUpdateCreate.ProjectUpdate

		if jsonOut {
			output.JSON(update)
		} else if plaintext {
			fmt.Printf("Posted status update %s\n", update.Id)
		} else {
			output.Success(fmt.Sprintf("Posted status update %s", update.Id), plaintext, jsonOut)
		}
	},
}

var projectUpdatePostEditCmd = &cobra.Command{
	Use:   "edit <update-id>",
	Short: "Edit a status update's body or health",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		updateID := args[0]

		input := api.ProjectUpdateUpdateInput{}
		if cmd.Flags().Changed("body") {
			body, _ := cmd.Flags().GetString("body")
			input.Body = &body
		}
		if cmd.Flags().Changed("health") {
			healthStr, _ := cmd.Flags().GetString("health")
			h, err := parseProjectUpdateHealth(healthStr)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.Health = &h
		}
		if input.Body == nil && input.Health == nil {
			output.Error("No updates specified. Use --body and/or --health.", plaintext, jsonOut)
			os.Exit(1)
		}

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		resp, err := api.ProjectUpdateUpdate(ctx, client, updateID, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to edit status update: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectUpdateUpdate.Success {
			output.Error("Failed to edit status update", plaintext, jsonOut)
			os.Exit(1)
		}

		update := resp.ProjectUpdateUpdate.ProjectUpdate

		if jsonOut {
			output.JSON(update)
		} else if plaintext {
			fmt.Printf("Updated status update %s\n", update.Id)
		} else {
			output.Success(fmt.Sprintf("Updated status update %s", update.Id), plaintext, jsonOut)
		}
	},
}

var projectUpdatePostArchiveCmd = &cobra.Command{
	Use:   "archive <update-id>",
	Short: "Archive a status update",
	Long:  `Archive a status update. This action executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		updateID := args[0]

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		resp, err := api.ProjectUpdateArchive(ctx, client, updateID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to archive status update: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectUpdateArchive.Success {
			output.Error("Failed to archive status update", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": updateID})
		} else if plaintext {
			fmt.Printf("Archived status update %s\n", updateID)
		} else {
			output.Success(fmt.Sprintf("Archived status update %s", updateID), plaintext, jsonOut)
		}
	},
}

var projectUpdatePostUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <update-id>",
	Short: "Unarchive a status update",
	Long:  `Unarchive a previously archived status update. This action executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		updateID := args[0]

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		resp, err := api.ProjectUpdateUnarchive(ctx, client, updateID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to unarchive status update: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectUpdateUnarchive.Success {
			output.Error("Failed to unarchive status update", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": updateID})
		} else if plaintext {
			fmt.Printf("Unarchived status update %s\n", updateID)
		} else {
			output.Success(fmt.Sprintf("Unarchived status update %s", updateID), plaintext, jsonOut)
		}
	},
}

var projectUpdateReminderCmd = &cobra.Command{
	Use:   "update-reminder <project-id-or-name>",
	Short: "Notify a user to post a project status update",
	Long: `Send a notification reminding a user to post a status update for a
project. Defaults to the project's lead if --user is not given.

Examples:
  lincli project update-reminder "My Project"
  lincli project update-reminder "My Project" --user jane@example.com`,
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		var userID *string
		if user, _ := cmd.Flags().GetString("user"); user != "" {
			id, err := resolveUser(ctx, client, cache, user)
			if err != nil {
				output.Error(fmt.Sprintf("Failed to find user '%s': %v", user, err), plaintext, jsonOut)
				os.Exit(1)
			}
			userID = &id
		}

		resp, err := api.CreateProjectUpdateReminder(ctx, client, projectID, userID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to send update reminder: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.CreateProjectUpdateReminder.Success {
			output.Error("Failed to send update reminder", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true})
		} else if plaintext {
			fmt.Println("Sent update reminder")
		} else {
			output.Success("Sent update reminder", plaintext, jsonOut)
		}
	},
}

// --- Project labels ---

var projectLabelCmd = &cobra.Command{
	Use:   "label",
	Short: "Manage the workspace's project label catalog",
	Long: `List, create, update, delete, retire, and restore project labels, and
add/remove a label on a specific project.

ProjectLabel is a distinct, workspace-wide catalog from IssueLabel and
InitiativeLabel - it has its own names/ids and is not shared with either.

Examples:
  lincli project label list
  lincli project label create --name "Design" --color "#5e6ad2"
  lincli project label add "My Project" "Design"
  lincli project label remove "My Project" "Design"
  lincli project label retire LABEL-ID
  lincli project label restore LABEL-ID`,
}

var projectLabelListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List the workspace's project labels",
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

		resp, err := api.ListProjectLabels(ctx, client, nil, limitPtr)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list project labels: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		labels := []*api.ListProjectLabelsProjectLabelsProjectLabelConnectionNodesProjectLabel{}
		if resp.ProjectLabels != nil {
			labels = resp.ProjectLabels.Nodes
		}

		if len(labels) == 0 {
			output.Info("No project labels found", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(labels)
			return
		}

		if plaintext {
			fmt.Println("# Project Labels")
			for _, l := range labels {
				fmt.Printf("## %s\n", l.Name)
				fmt.Printf("- **ID**: %s\n", l.Id)
				fmt.Printf("- **Color**: %s\n", l.Color)
				if l.Description != nil && *l.Description != "" {
					fmt.Printf("- **Description**: %s\n", *l.Description)
				}
				if l.RetiredAt != nil {
					fmt.Printf("- **Retired**: %s\n", l.RetiredAt.Format("2006-01-02"))
				}
				fmt.Println()
			}
			return
		}

		headers := []string{"Name", "ID", "Color", "Description", "Retired"}
		rows := make([][]string, len(labels))
		for i, l := range labels {
			description := ""
			if l.Description != nil {
				description = *l.Description
			}
			retired := ""
			if l.RetiredAt != nil {
				retired = l.RetiredAt.Format("2006-01-02")
			}
			rows[i] = []string{l.Name, l.Id, l.Color, description, retired}
		}
		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)
	},
}

var projectLabelCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a new project label",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			output.Error("Name is required (--name)", plaintext, jsonOut)
			os.Exit(1)
		}
		color, _ := cmd.Flags().GetString("color")
		if color == "" {
			output.Error("Color is required (--color)", plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.ProjectLabelCreateInput{Name: name, Color: &color}
		if description, _ := cmd.Flags().GetString("description"); description != "" {
			input.Description = &description
		}

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		resp, err := api.ProjectLabelCreate(ctx, client, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create project label: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectLabelCreate.Success || resp.ProjectLabelCreate.ProjectLabel == nil {
			output.Error("Failed to create project label", plaintext, jsonOut)
			os.Exit(1)
		}

		label := resp.ProjectLabelCreate.ProjectLabel

		if jsonOut {
			output.JSON(label)
		} else if plaintext {
			fmt.Printf("Created project label: %s\n", label.Name)
		} else {
			output.Success(fmt.Sprintf("Created project label: %s", label.Name), plaintext, jsonOut)
		}
	},
}

var projectLabelUpdateCmd = &cobra.Command{
	Use:   "update <label-id-or-name>",
	Short: "Update a project label",
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

		labelID, err := resolveProjectLabel(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.ProjectLabelUpdateInput{}
		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
		}
		if cmd.Flags().Changed("color") {
			color, _ := cmd.Flags().GetString("color")
			input.Color = &color
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
		}

		if input.Name == nil && input.Color == nil && input.Description == nil {
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectLabelUpdate(ctx, client, labelID, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update project label: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectLabelUpdate.Success || resp.ProjectLabelUpdate.ProjectLabel == nil {
			output.Error("Failed to update project label", plaintext, jsonOut)
			os.Exit(1)
		}

		label := resp.ProjectLabelUpdate.ProjectLabel

		if jsonOut {
			output.JSON(label)
		} else if plaintext {
			fmt.Printf("Updated project label: %s\n", label.Name)
		} else {
			output.Success(fmt.Sprintf("Updated project label: %s", label.Name), plaintext, jsonOut)
		}
	},
}

var projectLabelDeleteCmd = &cobra.Command{
	Use:   "delete <label-id-or-name>",
	Short: "Delete a project label",
	Long:  `Delete a project label. This action executes immediately with no confirmation prompt.`,
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

		labelID, err := resolveProjectLabel(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectLabelDelete(ctx, client, labelID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete project label: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectLabelDelete.Success {
			output.Error("Failed to delete project label", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.ProjectLabelDelete.EntityId})
		} else if plaintext {
			fmt.Printf("Deleted project label %s\n", resp.ProjectLabelDelete.EntityId)
		} else {
			output.Success(fmt.Sprintf("Deleted project label %s", resp.ProjectLabelDelete.EntityId), plaintext, jsonOut)
		}
	},
}

var projectLabelRetireCmd = &cobra.Command{
	Use:   "retire <label-id-or-name>",
	Short: "Retire a project label",
	Long:  `Retire a project label. Retired labels remain on projects that already use them but can't be applied to new ones. This action executes immediately with no confirmation prompt.`,
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

		labelID, err := resolveProjectLabel(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectLabelRetire(ctx, client, labelID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to retire project label: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectLabelRetire.Success {
			output.Error("Failed to retire project label", plaintext, jsonOut)
			os.Exit(1)
		}

		name := labelID
		if resp.ProjectLabelRetire.ProjectLabel != nil {
			name = resp.ProjectLabelRetire.ProjectLabel.Name
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": labelID})
		} else if plaintext {
			fmt.Printf("Retired project label %s\n", name)
		} else {
			output.Success(fmt.Sprintf("Retired project label %s", name), plaintext, jsonOut)
		}
	},
}

var projectLabelRestoreCmd = &cobra.Command{
	Use:   "restore <label-id-or-name>",
	Short: "Restore a previously retired project label",
	Long:  `Restore a previously retired project label, making it available for use on new projects. This action executes immediately with no confirmation prompt.`,
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

		labelID, err := resolveProjectLabel(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectLabelRestore(ctx, client, labelID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to restore project label: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectLabelRestore.Success {
			output.Error("Failed to restore project label", plaintext, jsonOut)
			os.Exit(1)
		}

		name := labelID
		if resp.ProjectLabelRestore.ProjectLabel != nil {
			name = resp.ProjectLabelRestore.ProjectLabel.Name
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": labelID})
		} else if plaintext {
			fmt.Printf("Restored project label %s\n", name)
		} else {
			output.Success(fmt.Sprintf("Restored project label %s", name), plaintext, jsonOut)
		}
	},
}

var projectLabelAddCmd = &cobra.Command{
	Use:   "add <project-id-or-name> <label-id-or-name>",
	Short: "Add a label to a project",
	Args:  cobra.ExactArgs(2),
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}
		labelID, err := resolveProjectLabel(ctx, client, cache, args[1])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectAddLabel(ctx, client, projectID, labelID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to add label to project: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectAddLabel.Success {
			output.Error("Failed to add label to project", plaintext, jsonOut)
			os.Exit(1)
		}

		// ProjectPayload.project is nullable in the schema, and confirmed
		// live: projectAddLabel returns success: true with project: null even
		// though the label is genuinely applied (visible on a subsequent
		// 'project get'). Fall back to the already-resolved project/label ids
		// rather than dereferencing a nil project.
		if resp.ProjectAddLabel.Project == nil {
			if jsonOut {
				output.JSON(map[string]interface{}{"success": true, "projectId": projectID, "labelId": labelID})
			} else if plaintext {
				fmt.Println("Added label to project")
			} else {
				output.Success("Added label to project", plaintext, jsonOut)
			}
			return
		}

		project := resp.ProjectAddLabel.Project

		if jsonOut {
			output.JSON(project)
		} else if plaintext {
			fmt.Printf("Added label to project %s\n", project.ProjectListFields.Name)
		} else {
			output.Success(fmt.Sprintf("Added label to project %s", project.ProjectListFields.Name), plaintext, jsonOut)
		}
	},
}

var projectLabelRemoveCmd = &cobra.Command{
	Use:     "remove <project-id-or-name> <label-id-or-name>",
	Aliases: []string{"rm"},
	Short:   "Remove a label from a project",
	Long:    `Remove a label from a project. This action executes immediately with no confirmation prompt.`,
	Args:    cobra.ExactArgs(2),
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}
		labelID, err := resolveProjectLabel(ctx, client, cache, args[1])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectRemoveLabel(ctx, client, projectID, labelID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to remove label from project: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectRemoveLabel.Success {
			output.Error("Failed to remove label from project", plaintext, jsonOut)
			os.Exit(1)
		}

		// See the matching comment in 'project label add': ProjectPayload.project
		// is nullable, and projectRemoveLabel is confirmed live to return
		// success: true with project: null even though the label is genuinely
		// removed.
		if resp.ProjectRemoveLabel.Project == nil {
			if jsonOut {
				output.JSON(map[string]interface{}{"success": true, "projectId": projectID, "labelId": labelID})
			} else if plaintext {
				fmt.Println("Removed label from project")
			} else {
				output.Success("Removed label from project", plaintext, jsonOut)
			}
			return
		}

		project := resp.ProjectRemoveLabel.Project

		if jsonOut {
			output.JSON(project)
		} else if plaintext {
			fmt.Printf("Removed label from project %s\n", project.ProjectListFields.Name)
		} else {
			output.Success(fmt.Sprintf("Removed label from project %s", project.ProjectListFields.Name), plaintext, jsonOut)
		}
	},
}

// --- Project relations ---

// findProjectRelation locates the ProjectRelation join record between
// projectID and otherID, checking both the outgoing `relations` connection
// (projectID is the source) and the incoming `inverseRelations` connection
// (projectID is the target), since 'project relate' may have been run with
// either project as the first argument. Project's relation connections take
// no filter for matching a specific related project, so both lists are
// paginated (up to `first`) and matched client-side.
func findProjectRelation(ctx context.Context, client graphql.Client, projectID, otherID string) (string, error) {
	first := 250
	resp, err := api.GetProjectRelations(ctx, client, projectID, &first)
	if err != nil {
		return "", fmt.Errorf("failed to look up project relations: %w", err)
	}
	if resp.Project == nil {
		return "", fmt.Errorf("project not found")
	}
	if resp.Project.Relations != nil {
		for _, node := range resp.Project.Relations.Nodes {
			if node.RelatedProject != nil && node.RelatedProject.Id == otherID {
				return node.Id, nil
			}
		}
	}
	if resp.Project.InverseRelations != nil {
		for _, node := range resp.Project.InverseRelations.Nodes {
			if node.Project != nil && node.Project.Id == otherID {
				return node.Id, nil
			}
		}
	}
	return "", fmt.Errorf("no relation found between these projects")
}

var projectRelateCmd = &cobra.Command{
	Use:   "relate <project-id-or-name> <other-project-id-or-name>",
	Short: "Create a finish-to-start dependency between two projects",
	Long: `Create a dependency relation between two projects: <project> must finish
before <other-project> starts.

Despite ProjectRelationCreateInput typing 'type'/'anchorType'/
'relatedAnchorType' as plain strings in schema.graphql (not GraphQL enums,
unlike IssueRelationType), the API enforces a fixed set of values for all
three server-side - confirmed live via INVALID_INPUT validation errors:
  - type: dependency (the only value currently accepted)
  - anchorType / relatedAnchorType: start, end, milestone
This command always anchors <project>'s end to <other-project>'s start
(a "finish-to-start" dependency); anchoring to a specific milestone isn't
exposed here.

Examples:
  lincli project relate "API" "Mobile App"`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		relTypeFlag, _ := cmd.Flags().GetString("type")
		if relTypeFlag == "" {
			relTypeFlag = "dependency"
		}
		relType, err := parseProjectRelationType(relTypeFlag)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}
		otherID, err := resolveProject(ctx, client, cache, args[1])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.ProjectRelationCreateInput{
			ProjectId:         projectID,
			RelatedProjectId:  otherID,
			AnchorType:        "end",
			RelatedAnchorType: "start",
			Type:              relType,
		}

		resp, err := api.ProjectRelationCreate(ctx, client, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create project relation: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectRelationCreate.Success {
			output.Error("Failed to create project relation", plaintext, jsonOut)
			os.Exit(1)
		}

		rel := resp.ProjectRelationCreate.ProjectRelation

		if jsonOut {
			output.JSON(rel)
		} else if plaintext {
			fmt.Printf("Related %s %s %s\n", rel.Project.Name, rel.Type, rel.RelatedProject.Name)
		} else {
			output.Success(fmt.Sprintf("Related %s %s %s", rel.Project.Name, rel.Type, rel.RelatedProject.Name), plaintext, jsonOut)
		}
	},
}

var projectUnrelateCmd = &cobra.Command{
	Use:   "unrelate <project-id-or-name> <other-project-id-or-name>",
	Short: "Remove a dependency relation between two projects",
	Long:  `Remove a dependency relation between two projects. This action executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(2),
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

		projectID, err := resolveProject(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}
		otherID, err := resolveProject(ctx, client, cache, args[1])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		relID, err := findProjectRelation(ctx, client, projectID, otherID)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectRelationDelete(ctx, client, relID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to remove project relation: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectRelationDelete.Success {
			output.Error("Failed to remove project relation", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.ProjectRelationDelete.EntityId})
		} else if plaintext {
			fmt.Println("Removed project relation")
		} else {
			output.Success("Removed project relation", plaintext, jsonOut)
		}
	},
}

// --- Project statuses ---

var projectStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Manage the workspace's project status catalog",
	Long: `List, create, update, archive, and unarchive project statuses: the
workspace-wide catalog a project's --status flag resolves against (see
resolveProjectStatus).

Examples:
  lincli project status list
  lincli project status create --name "In Review" --type started
  lincli project status update STATUS-ID --color "#f2c94c"
  lincli project status archive STATUS-ID`,
}

var projectStatusListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List the workspace's project statuses",
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

		limit := 250
		resp, err := api.ListProjectStatuses(ctx, client, &limit)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list project statuses: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		statuses := []*api.ListProjectStatusesProjectStatusesProjectStatusConnectionNodesProjectStatus{}
		if resp.ProjectStatuses != nil {
			statuses = resp.ProjectStatuses.Nodes
		}

		if len(statuses) == 0 {
			output.Info("No project statuses found", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(statuses)
			return
		}

		if plaintext {
			fmt.Println("# Project Statuses")
			for _, s := range statuses {
				fmt.Printf("## %s\n", s.Name)
				fmt.Printf("- **ID**: %s\n", s.Id)
				fmt.Printf("- **Type**: %s\n", s.Type)
				fmt.Printf("- **Color**: %s\n", s.Color)
				fmt.Printf("- **Position**: %.0f\n", s.Position)
				if s.Description != nil && *s.Description != "" {
					fmt.Printf("- **Description**: %s\n", *s.Description)
				}
				fmt.Println()
			}
			return
		}

		headers := []string{"Name", "ID", "Type", "Color", "Position"}
		rows := make([][]string, len(statuses))
		for i, s := range statuses {
			rows[i] = []string{s.Name, s.Id, string(s.Type), s.Color, fmt.Sprintf("%.0f", s.Position)}
		}
		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)
	},
}

var projectStatusCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a new project status",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			output.Error("Name is required (--name)", plaintext, jsonOut)
			os.Exit(1)
		}
		typeStr, _ := cmd.Flags().GetString("type")
		if typeStr == "" {
			output.Error(fmt.Sprintf("Type is required (--type). Valid values: %s", strings.Join(projectStatusTypeValues, ", ")), plaintext, jsonOut)
			os.Exit(1)
		}
		statusType, err := parseProjectStatusType(typeStr)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		color, _ := cmd.Flags().GetString("color")
		position, _ := cmd.Flags().GetFloat64("position")

		input := api.ProjectStatusCreateInput{
			Name:     name,
			Type:     statusType,
			Color:    color,
			Position: position,
		}
		if description, _ := cmd.Flags().GetString("description"); description != "" {
			input.Description = &description
		}

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		resp, err := api.ProjectStatusCreate(ctx, client, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create project status: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectStatusCreate.Success || resp.ProjectStatusCreate.Status == nil {
			output.Error("Failed to create project status", plaintext, jsonOut)
			os.Exit(1)
		}

		status := resp.ProjectStatusCreate.Status

		if jsonOut {
			output.JSON(status)
		} else if plaintext {
			fmt.Printf("Created project status: %s\n", status.Name)
		} else {
			output.Success(fmt.Sprintf("Created project status: %s", status.Name), plaintext, jsonOut)
		}
	},
}

var projectStatusUpdateCmd = &cobra.Command{
	Use:   "update <status-id-or-name>",
	Short: "Update a project status",
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

		statusID, err := resolveProjectStatus(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.ProjectStatusUpdateInput{}
		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
		}
		if cmd.Flags().Changed("type") {
			typeStr, _ := cmd.Flags().GetString("type")
			statusType, err := parseProjectStatusType(typeStr)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.Type = &statusType
		}
		if cmd.Flags().Changed("color") {
			color, _ := cmd.Flags().GetString("color")
			input.Color = &color
		}
		if cmd.Flags().Changed("position") {
			position, _ := cmd.Flags().GetFloat64("position")
			input.Position = &position
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
		}

		if input.Name == nil && input.Type == nil && input.Color == nil && input.Position == nil && input.Description == nil {
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectStatusUpdate(ctx, client, statusID, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update project status: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectStatusUpdate.Success || resp.ProjectStatusUpdate.Status == nil {
			output.Error("Failed to update project status", plaintext, jsonOut)
			os.Exit(1)
		}

		status := resp.ProjectStatusUpdate.Status

		if jsonOut {
			output.JSON(status)
		} else if plaintext {
			fmt.Printf("Updated project status: %s\n", status.Name)
		} else {
			output.Success(fmt.Sprintf("Updated project status: %s", status.Name), plaintext, jsonOut)
		}
	},
}

var projectStatusArchiveCmd = &cobra.Command{
	Use:   "archive <status-id-or-name>",
	Short: "Archive a project status",
	Long:  `Archive a project status. The status must not have any active projects assigned to it and must not be the last status of its type. This action executes immediately with no confirmation prompt.`,
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

		statusID, err := resolveProjectStatus(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectStatusArchive(ctx, client, statusID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to archive project status: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectStatusArchive.Success {
			output.Error("Failed to archive project status", plaintext, jsonOut)
			os.Exit(1)
		}

		name := statusID
		if resp.ProjectStatusArchive.Entity != nil {
			name = resp.ProjectStatusArchive.Entity.Name
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": statusID})
		} else if plaintext {
			fmt.Printf("Archived project status %s\n", name)
		} else {
			output.Success(fmt.Sprintf("Archived project status %s", name), plaintext, jsonOut)
		}
	},
}

var projectStatusUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <status-id-or-name>",
	Short: "Unarchive a project status",
	Long:  `Unarchive a previously archived project status. This action executes immediately with no confirmation prompt.`,
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

		statusID, err := resolveProjectStatus(ctx, client, cache, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectStatusUnarchive(ctx, client, statusID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to unarchive project status: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectStatusUnarchive.Success {
			output.Error("Failed to unarchive project status", plaintext, jsonOut)
			os.Exit(1)
		}

		name := statusID
		if resp.ProjectStatusUnarchive.Entity != nil {
			name = resp.ProjectStatusUnarchive.Entity.Name
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": statusID})
		} else if plaintext {
			fmt.Printf("Unarchived project status %s\n", name)
		} else {
			output.Success(fmt.Sprintf("Unarchived project status %s", name), plaintext, jsonOut)
		}
	},
}

var projectReassignStatusCmd = &cobra.Command{
	Use:   "reassign-status",
	Short: "Move all projects from one status to another",
	Long: `[INTERNAL] Reassign every project currently on one status to a
different status. Typically used before archiving or deleting a status that
still has projects assigned to it.

Examples:
  lincli project reassign-status --from "In Review" --to "In Progress"`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		if from == "" || to == "" {
			output.Error("Both --from and --to are required", plaintext, jsonOut)
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

		fromID, err := resolveProjectStatus(ctx, client, cache, from)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}
		toID, err := resolveProjectStatus(ctx, client, cache, to)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ProjectReassignStatus(ctx, client, fromID, toID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to reassign project status: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.ProjectReassignStatus.Success {
			output.Error("Failed to reassign project status", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true})
		} else if plaintext {
			fmt.Printf("Reassigned projects from %s to %s\n", from, to)
		} else {
			output.Success(fmt.Sprintf("Reassigned projects from %s to %s", from, to), plaintext, jsonOut)
		}
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectGetCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectUpdateCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectUnarchiveCmd)

	// Members
	projectCmd.AddCommand(projectMembersCmd)
	projectCmd.AddCommand(projectMemberCmd)
	projectMemberCmd.AddCommand(projectMemberAddCmd)
	projectMemberCmd.AddCommand(projectMemberRemoveCmd)

	// Milestones
	projectCmd.AddCommand(projectMilestoneCmd)
	projectMilestoneCmd.AddCommand(projectMilestoneListCmd)
	projectMilestoneCmd.AddCommand(projectMilestoneGetCmd)
	projectMilestoneCmd.AddCommand(projectMilestoneCreateCmd)
	projectMilestoneCmd.AddCommand(projectMilestoneUpdateCmd)
	projectMilestoneCmd.AddCommand(projectMilestoneMoveCmd)
	projectMilestoneCmd.AddCommand(projectMilestoneDeleteCmd)

	// List command flags
	projectListCmd.Flags().StringP("team", "t", "", "Filter by team key")
	projectListCmd.Flags().StringP("state", "s", "", "Filter by state (planned, started, paused, completed, canceled)")
	projectListCmd.Flags().IntP("limit", "l", 50, "Maximum number of projects to return")
	projectListCmd.Flags().BoolP("include-completed", "c", false, "Include completed and canceled projects")
	projectListCmd.Flags().StringP("sort", "o", "linear", "Sort order: linear (default), created, updated")
	projectListCmd.Flags().StringP("newer-than", "n", "", "Show projects created after this time (default: 6_months_ago, use 'all_time' for no filter)")

	// Create command flags
	projectCreateCmd.Flags().String("name", "", "Project name (required)")
	projectCreateCmd.Flags().StringP("team", "t", "", "Team key, name, or ID (required)")
	projectCreateCmd.Flags().StringP("description", "d", "", "Project description")
	projectCreateCmd.Flags().String("lead", "", "Lead email, name, 'me', or user ID")
	projectCreateCmd.Flags().String("start-date", "", "Planned start date (YYYY-MM-DD)")
	projectCreateCmd.Flags().String("target-date", "", "Planned target date (YYYY-MM-DD)")
	projectCreateCmd.Flags().String("status", "", "Project status name or ID")
	projectCreateCmd.Flags().String("template", "", "Project template name or ID to apply")
	_ = projectCreateCmd.MarkFlagRequired("name")
	_ = projectCreateCmd.MarkFlagRequired("team")

	// Update command flags
	projectUpdateCmd.Flags().String("name", "", "New project name")
	projectUpdateCmd.Flags().StringP("description", "d", "", "New description")
	projectUpdateCmd.Flags().String("lead", "", "New lead (email, name, 'me', user ID, or 'unassigned' to clear)")
	projectUpdateCmd.Flags().String("start-date", "", "New start date (YYYY-MM-DD, or empty string to clear)")
	projectUpdateCmd.Flags().String("target-date", "", "New target date (YYYY-MM-DD, or empty string to clear)")
	projectUpdateCmd.Flags().String("status", "", "New project status name or ID")

	// Milestone list command flags
	projectMilestoneListCmd.Flags().IntP("limit", "l", 50, "Maximum number of milestones to return")

	// Milestone get/update/delete: --project scopes name resolution when the
	// milestone argument isn't already an ID.
	projectMilestoneGetCmd.Flags().String("project", "", "Project name or ID (required if <milestone> is a name, not an ID)")
	projectMilestoneUpdateCmd.Flags().String("project", "", "Project name or ID (required if <milestone> is a name, not an ID)")
	projectMilestoneDeleteCmd.Flags().String("project", "", "Project name or ID (required if <milestone> is a name, not an ID)")

	// Milestone create command flags
	projectMilestoneCreateCmd.Flags().String("name", "", "Milestone name (required)")
	projectMilestoneCreateCmd.Flags().StringP("description", "d", "", "Milestone description")
	projectMilestoneCreateCmd.Flags().String("target-date", "", "Planned target date (YYYY-MM-DD)")
	projectMilestoneCreateCmd.Flags().Float64("sort-order", 0, "Sort order within the project")
	_ = projectMilestoneCreateCmd.MarkFlagRequired("name")

	// Milestone update command flags
	projectMilestoneUpdateCmd.Flags().String("name", "", "New milestone name")
	projectMilestoneUpdateCmd.Flags().StringP("description", "d", "", "New description")
	projectMilestoneUpdateCmd.Flags().String("target-date", "", "New target date (YYYY-MM-DD, or empty string to clear)")
	projectMilestoneUpdateCmd.Flags().Float64("sort-order", 0, "New sort order within the project")

	// Milestone move command flags
	projectMilestoneMoveCmd.Flags().String("project", "", "Destination project name or ID (required)")
	projectMilestoneMoveCmd.Flags().Float64("sort-order", 0, "New sort order within the destination project")
	_ = projectMilestoneMoveCmd.MarkFlagRequired("project")

	// Status-update posts
	projectCmd.AddCommand(projectUpdatePostCmd)
	projectUpdatePostCmd.AddCommand(projectUpdatePostListCmd)
	projectUpdatePostCmd.AddCommand(projectUpdatePostCreateCmd)
	projectUpdatePostCmd.AddCommand(projectUpdatePostEditCmd)
	projectUpdatePostCmd.AddCommand(projectUpdatePostArchiveCmd)
	projectUpdatePostCmd.AddCommand(projectUpdatePostUnarchiveCmd)
	projectCmd.AddCommand(projectUpdateReminderCmd)

	// Update-post list command flags
	projectUpdatePostListCmd.Flags().IntP("limit", "l", 50, "Maximum number of status updates to return")

	// Update-post create command flags
	projectUpdatePostCreateCmd.Flags().String("body", "", "Status update body in markdown (required)")
	projectUpdatePostCreateCmd.Flags().String("health", "", fmt.Sprintf("Project health at the time of the update: %s", strings.Join(projectUpdateHealthValues, ", ")))
	_ = projectUpdatePostCreateCmd.MarkFlagRequired("body")

	// Update-post edit command flags
	projectUpdatePostEditCmd.Flags().String("body", "", "New status update body in markdown")
	projectUpdatePostEditCmd.Flags().String("health", "", fmt.Sprintf("New health value: %s", strings.Join(projectUpdateHealthValues, ", ")))

	// Update-reminder command flags
	projectUpdateReminderCmd.Flags().String("user", "", "User email, name, 'me', or ID to remind (defaults to the project lead)")

	// Labels
	projectCmd.AddCommand(projectLabelCmd)
	projectLabelCmd.AddCommand(projectLabelListCmd)
	projectLabelCmd.AddCommand(projectLabelCreateCmd)
	projectLabelCmd.AddCommand(projectLabelUpdateCmd)
	projectLabelCmd.AddCommand(projectLabelDeleteCmd)
	projectLabelCmd.AddCommand(projectLabelRetireCmd)
	projectLabelCmd.AddCommand(projectLabelRestoreCmd)
	projectLabelCmd.AddCommand(projectLabelAddCmd)
	projectLabelCmd.AddCommand(projectLabelRemoveCmd)

	// Label list command flags
	projectLabelListCmd.Flags().IntP("limit", "l", 250, "Maximum number of project labels to return")

	// Label create command flags
	projectLabelCreateCmd.Flags().String("name", "", "Label name (required)")
	projectLabelCreateCmd.Flags().String("color", "", "Label color as a HEX string, e.g. #5e6ad2 (required)")
	projectLabelCreateCmd.Flags().StringP("description", "d", "", "Label description")
	_ = projectLabelCreateCmd.MarkFlagRequired("name")
	_ = projectLabelCreateCmd.MarkFlagRequired("color")

	// Label update command flags
	projectLabelUpdateCmd.Flags().String("name", "", "New label name")
	projectLabelUpdateCmd.Flags().String("color", "", "New label color as a HEX string")
	projectLabelUpdateCmd.Flags().StringP("description", "d", "", "New description")

	// Relations
	projectCmd.AddCommand(projectRelateCmd)
	projectCmd.AddCommand(projectUnrelateCmd)
	projectRelateCmd.Flags().String("type", "dependency", fmt.Sprintf("Relation type: %s", strings.Join(projectRelationTypeValues, ", ")))

	// Statuses
	projectCmd.AddCommand(projectStatusCmd)
	projectStatusCmd.AddCommand(projectStatusListCmd)
	projectStatusCmd.AddCommand(projectStatusCreateCmd)
	projectStatusCmd.AddCommand(projectStatusUpdateCmd)
	projectStatusCmd.AddCommand(projectStatusArchiveCmd)
	projectStatusCmd.AddCommand(projectStatusUnarchiveCmd)
	projectCmd.AddCommand(projectReassignStatusCmd)

	// Status create command flags
	projectStatusCreateCmd.Flags().String("name", "", "Status name (required)")
	projectStatusCreateCmd.Flags().String("type", "", fmt.Sprintf("Status type (required): %s", strings.Join(projectStatusTypeValues, ", ")))
	projectStatusCreateCmd.Flags().String("color", "#bec2c8", "Status color as a HEX string")
	projectStatusCreateCmd.Flags().Float64("position", 0, "Position within the workspace's project flow")
	projectStatusCreateCmd.Flags().StringP("description", "d", "", "Status description")
	_ = projectStatusCreateCmd.MarkFlagRequired("name")
	_ = projectStatusCreateCmd.MarkFlagRequired("type")

	// Status update command flags
	projectStatusUpdateCmd.Flags().String("name", "", "New status name")
	projectStatusUpdateCmd.Flags().String("type", "", fmt.Sprintf("New status type: %s", strings.Join(projectStatusTypeValues, ", ")))
	projectStatusUpdateCmd.Flags().String("color", "", "New status color as a HEX string")
	projectStatusUpdateCmd.Flags().Float64("position", 0, "New position within the workspace's project flow")
	projectStatusUpdateCmd.Flags().StringP("description", "d", "", "New description")

	// Reassign-status command flags
	projectReassignStatusCmd.Flags().String("from", "", "Status name or ID to move projects off of (required)")
	projectReassignStatusCmd.Flags().String("to", "", "Status name or ID to move projects onto (required)")
	_ = projectReassignStatusCmd.MarkFlagRequired("from")
	_ = projectReassignStatusCmd.MarkFlagRequired("to")
}

// buildProjectFilterTyped builds a typed ProjectFilter from command flags
func buildProjectFilterTyped(cmd *cobra.Command) api.ProjectFilter {
	filter := api.ProjectFilter{}

	// State filter
	state, _ := cmd.Flags().GetString("state")
	if state != "" {
		filter.State = &api.StringComparator{Eq: &state}
	} else {
		// Exclude completed/canceled unless explicitly included
		includeCompleted, _ := cmd.Flags().GetBool("include-completed")
		if !includeCompleted {
			filter.State = &api.StringComparator{
				Nin: []string{"completed", "canceled"},
			}
		}
	}

	// Time filter
	newerThan, _ := cmd.Flags().GetString("newer-than")
	createdAt, err := utils.ParseTimeExpression(newerThan)
	if err != nil {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
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
