package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/shanedolley/lincli/pkg/utils"
	"github.com/fatih/color"
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
					fmt.Printf("- **Color**: %s\n", doc.Color)
					fmt.Printf("- **Created**: %s by %s\n", doc.CreatedAt.Format("2006-01-02"), doc.Creator.Name)
					if doc.UpdatedBy != nil {
						fmt.Printf("- **Updated**: %s by %s\n", doc.UpdatedAt.Format("2006-01-02"), doc.UpdatedBy.Name)
					}
					fmt.Printf("\n%s\n", doc.Content)
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

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectGetCmd)

	// List command flags
	projectListCmd.Flags().StringP("team", "t", "", "Filter by team key")
	projectListCmd.Flags().StringP("state", "s", "", "Filter by state (planned, started, paused, completed, canceled)")
	projectListCmd.Flags().IntP("limit", "l", 50, "Maximum number of projects to return")
	projectListCmd.Flags().BoolP("include-completed", "c", false, "Include completed and canceled projects")
	projectListCmd.Flags().StringP("sort", "o", "linear", "Sort order: linear (default), created, updated")
	projectListCmd.Flags().StringP("newer-than", "n", "", "Show projects created after this time (default: 6_months_ago, use 'all_time' for no filter)")
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
