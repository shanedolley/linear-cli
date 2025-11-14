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

// issueCmd represents the issue command
var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Manage Linear issues",
	Long: `Create, list, update, and manage Linear issues.

Examples:
  lincli issue list --assignee me --state "In Progress"
  lincli issue ls -a me -s "In Progress"
  lincli issue list --include-completed  # Show all issues including completed
  lincli issue list --newer-than 3_weeks_ago  # Show issues from last 3 weeks
  lincli issue search "login bug" --team ENG
  lincli issue get LIN-123
  lincli issue create --title "Bug fix" --team ENG`,
}

var issueListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List issues",
	Long:    `List Linear issues with optional filtering.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error("Not authenticated. Run 'lincli auth' first.", plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)

		// Build typed filter from flags
		filterTyped := buildIssueFilterTyped(cmd)

		limit, _ := cmd.Flags().GetInt("limit")
		if limit == 0 {
			limit = 50
		}

		// Get sort option and convert to enum
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

		resp, err := api.ListIssues(context.Background(), client, filterTyped, limitPtr, nil, orderByEnum)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to fetch issues: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		// Check if empty
		if len(resp.Issues.Nodes) == 0 {
			output.Info("No issues found", plaintext, jsonOut)
			return
		}

		// JSON output
		if jsonOut {
			output.JSON(resp.Issues.Nodes)
			return
		}

		// Plaintext output
		if plaintext {
			fmt.Println("# Issues")
			for _, node := range resp.Issues.Nodes {
				f := node.IssueListFields
				fmt.Printf("## %s\n", f.Title)
				fmt.Printf("- **ID**: %s\n", f.Identifier)
				if f.State != nil {
					fmt.Printf("- **State**: %s\n", f.State.Name)
				}
				if f.Assignee != nil {
					fmt.Printf("- **Assignee**: %s\n", f.Assignee.Name)
				} else {
					fmt.Printf("- **Assignee**: Unassigned\n")
				}
				if f.Team != nil {
					fmt.Printf("- **Team**: %s\n", f.Team.Key)
				}
				fmt.Printf("- **Created**: %s\n", f.CreatedAt.Format("2006-01-02"))
				fmt.Printf("- **URL**: %s\n", f.Url)
				if f.Description != nil && *f.Description != "" {
					fmt.Printf("- **Description**: %s\n", *f.Description)
				}
				fmt.Println()
			}
			fmt.Printf("\nTotal: %d issues\n", len(resp.Issues.Nodes))
			return
		}

		// Table output
		headers := []string{"Title", "State", "Assignee", "Team", "Created", "URL"}
		rows := make([][]string, len(resp.Issues.Nodes))

		for i, node := range resp.Issues.Nodes {
			f := node.IssueListFields

			assignee := "Unassigned"
			if f.Assignee != nil {
				assignee = f.Assignee.Name
			}

			team := ""
			if f.Team != nil {
				team = f.Team.Key
			}

			state := ""
			if f.State != nil {
				state = f.State.Name
			}

			rows[i] = []string{
				truncateString(f.Title, 50),
				state,
				assignee,
				team,
				f.CreatedAt.Format("2006-01-02"),
				f.Url,
			}
		}

		tableData := output.TableData{
			Headers: headers,
			Rows:    rows,
		}

		output.Table(tableData, false, false)
		fmt.Printf("\nTotal: %d issues\n", len(resp.Issues.Nodes))
	},
}

func renderIssueCollection(issues *api.Issues, plaintext, jsonOut bool, emptyMessage, summaryLabel, plaintextTitle string) {
	if len(issues.Nodes) == 0 {
		output.Info(emptyMessage, plaintext, jsonOut)
		return
	}

	if jsonOut {
		output.JSON(issues.Nodes)
		return
	}

	if plaintext {
		fmt.Println(plaintextTitle)
		for _, issue := range issues.Nodes {
			fmt.Printf("## %s\n", issue.Title)
			fmt.Printf("- **ID**: %s\n", issue.Identifier)
			if issue.State != nil {
				fmt.Printf("- **State**: %s\n", issue.State.Name)
			}
			if issue.Assignee != nil {
				fmt.Printf("- **Assignee**: %s\n", issue.Assignee.Name)
			} else {
				fmt.Printf("- **Assignee**: Unassigned\n")
			}
			if issue.Team != nil {
				fmt.Printf("- **Team**: %s\n", issue.Team.Key)
			}
			fmt.Printf("- **Created**: %s\n", issue.CreatedAt.Format("2006-01-02"))
			fmt.Printf("- **URL**: %s\n", issue.URL)
			if issue.Description != "" {
				fmt.Printf("- **Description**: %s\n", issue.Description)
			}
			fmt.Println()
		}
		fmt.Printf("\nTotal: %d %s\n", len(issues.Nodes), summaryLabel)
		return
	}

	headers := []string{"Title", "State", "Assignee", "Team", "Created", "URL"}
	rows := make([][]string, len(issues.Nodes))

	for i, issue := range issues.Nodes {
		assignee := "Unassigned"
		if issue.Assignee != nil {
			assignee = issue.Assignee.Name
		}

		team := ""
		if issue.Team != nil {
			team = issue.Team.Key
		}

		state := ""
		if issue.State != nil {
			state = issue.State.Name
			var stateColor *color.Color
			switch issue.State.Type {
			case "triage":
				stateColor = color.New(color.FgMagenta)
			case "backlog":
				stateColor = color.New(color.FgCyan)
			case "unstarted":
				stateColor = color.New(color.FgWhite)
			case "started":
				stateColor = color.New(color.FgBlue)
			case "completed":
				stateColor = color.New(color.FgGreen)
			case "canceled":
				stateColor = color.New(color.FgRed)
			default:
				stateColor = color.New(color.FgWhite)
			}
			state = stateColor.Sprint(state)
		}

		if issue.Assignee == nil {
			assignee = color.New(color.FgYellow).Sprint(assignee)
		}

		rows[i] = []string{
			truncateString(issue.Title, 40),
			state,
			assignee,
			team,
			issue.CreatedAt.Format("2006-01-02"),
			issue.URL,
		}
	}

	tableData := output.TableData{
		Headers: headers,
		Rows:    rows,
	}

	output.Table(tableData, false, false)

	fmt.Printf("\n%s %d %s\n",
		color.New(color.FgGreen).Sprint("✓"),
		len(issues.Nodes),
		summaryLabel)

	if issues.PageInfo.HasNextPage {
		fmt.Printf("%s Use --limit to see more results\n",
			color.New(color.FgYellow).Sprint("ℹ️"))
	}
}

var issueSearchCmd = &cobra.Command{
	Use:     "search [query]",
	Aliases: []string{"find"},
	Short:   "Search issues by keyword",
	Long: `Perform a full-text search across Linear issues.

Examples:
  lincli issue search "payment outage"
  lincli issue search "auth token" --team ENG --include-completed
  lincli issue search "customer:" --json`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		query := strings.TrimSpace(strings.Join(args, " "))
		if query == "" {
			output.Error("Search query is required", plaintext, jsonOut)
			os.Exit(1)
		}

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error("Not authenticated. Run 'lincli auth' first.", plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)

		// Build typed filter from flags
		filterTyped := buildIssueFilterTyped(cmd)

		limit, _ := cmd.Flags().GetInt("limit")
		if limit == 0 {
			limit = 50
		}

		// Get sort option and convert to enum
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

		includeArchived, _ := cmd.Flags().GetBool("include-archived")
		includeArchivedPtr := &includeArchived

		resp, err := api.SearchIssues(context.Background(), client, query, filterTyped, limitPtr, nil, orderByEnum, includeArchivedPtr)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to search issues: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		// Check if empty
		if len(resp.SearchIssues.Nodes) == 0 {
			output.Info(fmt.Sprintf("No matches found for %q", query), plaintext, jsonOut)
			return
		}

		// JSON output
		if jsonOut {
			output.JSON(resp.SearchIssues.Nodes)
			return
		}

		// Plaintext output
		if plaintext {
			fmt.Println("# Search Results")
			for _, node := range resp.SearchIssues.Nodes {
				fmt.Printf("## %s\n", node.Title)
				fmt.Printf("- **ID**: %s\n", node.Identifier)
				if node.State != nil {
					fmt.Printf("- **State**: %s\n", node.State.Name)
				}
				if node.Assignee != nil {
					fmt.Printf("- **Assignee**: %s\n", node.Assignee.Name)
				} else {
					fmt.Printf("- **Assignee**: Unassigned\n")
				}
				if node.Team != nil {
					fmt.Printf("- **Team**: %s\n", node.Team.Key)
				}
				fmt.Printf("- **Created**: %s\n", node.CreatedAt.Format("2006-01-02"))
				fmt.Printf("- **URL**: %s\n", node.Url)
				if node.Description != nil && *node.Description != "" {
					fmt.Printf("- **Description**: %s\n", *node.Description)
				}
				fmt.Println()
			}
			fmt.Printf("\nTotal: %d search results\n", len(resp.SearchIssues.Nodes))
			return
		}

		// Table output
		headers := []string{"Title", "State", "Assignee", "Team", "Created", "URL"}
		rows := make([][]string, len(resp.SearchIssues.Nodes))

		for i, node := range resp.SearchIssues.Nodes {
			assignee := "Unassigned"
			if node.Assignee != nil {
				assignee = node.Assignee.Name
			}

			team := ""
			if node.Team != nil {
				team = node.Team.Key
			}

			state := ""
			if node.State != nil {
				state = node.State.Name
			}

			rows[i] = []string{
				truncateString(node.Title, 50),
				state,
				assignee,
				team,
				node.CreatedAt.Format("2006-01-02"),
				node.Url,
			}
		}

		tableData := output.TableData{
			Headers: headers,
			Rows:    rows,
		}

		output.Table(tableData, false, false)
		fmt.Printf("\nTotal: %d search results\n", len(resp.SearchIssues.Nodes))
	},
}

var issueGetCmd = &cobra.Command{
	Use:     "get [issue-id]",
	Aliases: []string{"show"},
	Short:   "Get issue details",
	Long:    `Get detailed information about a specific issue.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error("Not authenticated. Run 'lincli auth' first.", plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		resp, err := api.GetIssue(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to fetch issue: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		issue := resp.Issue

		if jsonOut {
			output.JSON(issue.IssueDetailFields)
			return
		}

		if plaintext {
			fmt.Printf("# %s - %s\n\n", issue.IssueDetailFields.Identifier, issue.IssueDetailFields.Title)

			if issue.IssueDetailFields.Description != nil && *issue.IssueDetailFields.Description != "" {
				fmt.Printf("## Description\n%s\n\n", *issue.IssueDetailFields.Description)
			}

			fmt.Printf("## Core Details\n")
			fmt.Printf("- **ID**: %s\n", issue.IssueDetailFields.Identifier)
			fmt.Printf("- **Number**: %.0f\n", issue.IssueDetailFields.Number)
			if issue.IssueDetailFields.State != nil {
				fmt.Printf("- **State**: %s (%s)\n", issue.IssueDetailFields.State.Name, issue.IssueDetailFields.State.Type)
				if issue.IssueDetailFields.State.Description != nil && *issue.IssueDetailFields.State.Description != "" {
					fmt.Printf("  - Description: %s\n", *issue.IssueDetailFields.State.Description)
				}
			}
			if issue.IssueDetailFields.Assignee != nil {
				fmt.Printf("- **Assignee**: %s (%s)\n", issue.IssueDetailFields.Assignee.Name, issue.IssueDetailFields.Assignee.Email)
				if issue.IssueDetailFields.Assignee.DisplayName != "" && issue.IssueDetailFields.Assignee.DisplayName != issue.IssueDetailFields.Assignee.Name {
					fmt.Printf("  - Display Name: %s\n", issue.IssueDetailFields.Assignee.DisplayName)
				}
			} else {
				fmt.Printf("- **Assignee**: Unassigned\n")
			}
			if issue.IssueDetailFields.Creator != nil {
				fmt.Printf("- **Creator**: %s (%s)\n", issue.IssueDetailFields.Creator.Name, issue.IssueDetailFields.Creator.Email)
			}
			if issue.IssueDetailFields.Team != nil {
				fmt.Printf("- **Team**: %s (%s)\n", issue.IssueDetailFields.Team.Name, issue.IssueDetailFields.Team.Key)
				if issue.IssueDetailFields.Team.Description != nil && *issue.IssueDetailFields.Team.Description != "" {
					fmt.Printf("  - Description: %s\n", *issue.IssueDetailFields.Team.Description)
				}
			}
			fmt.Printf("- **Priority**: %s (%.0f)\n", priorityToString(int(issue.IssueDetailFields.Priority)), int(issue.IssueDetailFields.Priority))
			if issue.IssueDetailFields.PriorityLabel != "" {
				fmt.Printf("- **Priority Label**: %s\n", issue.IssueDetailFields.PriorityLabel)
			}
			if issue.IssueDetailFields.Estimate != nil {
				fmt.Printf("- **Estimate**: %.1f\n", *issue.IssueDetailFields.Estimate)
			}

			fmt.Printf("\n## Status & Dates\n")
			fmt.Printf("- **Created**: %s\n", issue.IssueDetailFields.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("- **Updated**: %s\n", issue.IssueDetailFields.UpdatedAt.Format("2006-01-02 15:04:05"))
			if issue.IssueDetailFields.TriagedAt != nil {
				fmt.Printf("- **Triaged**: %s\n", issue.IssueDetailFields.TriagedAt.Format("2006-01-02 15:04:05"))
			}
			if issue.IssueDetailFields.CompletedAt != nil {
				fmt.Printf("- **Completed**: %s\n", issue.IssueDetailFields.CompletedAt.Format("2006-01-02 15:04:05"))
			}
			if issue.IssueDetailFields.CanceledAt != nil {
				fmt.Printf("- **Canceled**: %s\n", issue.IssueDetailFields.CanceledAt.Format("2006-01-02 15:04:05"))
			}
			if issue.IssueDetailFields.ArchivedAt != nil {
				fmt.Printf("- **Archived**: %s\n", issue.IssueDetailFields.ArchivedAt.Format("2006-01-02 15:04:05"))
			}
			if issue.IssueDetailFields.DueDate != nil && *issue.IssueDetailFields.DueDate != "" {
				fmt.Printf("- **Due Date**: %s\n", *issue.IssueDetailFields.DueDate)
			}
			if issue.IssueDetailFields.SnoozedUntilAt != nil {
				fmt.Printf("- **Snoozed Until**: %s\n", issue.IssueDetailFields.SnoozedUntilAt.Format("2006-01-02 15:04:05"))
			}

			fmt.Printf("\n## Technical Details\n")
			fmt.Printf("- **Board Order**: %.2f\n", issue.IssueDetailFields.BoardOrder)
			if issue.IssueDetailFields.SubIssueSortOrder != nil {
				fmt.Printf("- **Sub-Issue Sort Order**: %.2f\n", *issue.IssueDetailFields.SubIssueSortOrder)
			}
			if issue.IssueDetailFields.BranchName != "" {
				fmt.Printf("- **Git Branch**: %s\n", issue.IssueDetailFields.BranchName)
			}
			if issue.IssueDetailFields.CustomerTicketCount > 0 {
				fmt.Printf("- **Customer Ticket Count**: %d\n", issue.IssueDetailFields.CustomerTicketCount)
			}
			if len(issue.IssueDetailFields.PreviousIdentifiers) > 0 {
				fmt.Printf("- **Previous Identifiers**: %s\n", strings.Join(issue.IssueDetailFields.PreviousIdentifiers, ", "))
			}
			if issue.IssueDetailFields.IntegrationSourceType != nil {
				fmt.Printf("- **Integration Source**: %s\n", *issue.IssueDetailFields.IntegrationSourceType)
			}
			if issue.IssueDetailFields.ExternalUserCreator != nil {
				fmt.Printf("- **External Creator**: %s (%s)\n", issue.IssueDetailFields.ExternalUserCreator.Name, issue.IssueDetailFields.ExternalUserCreator.Email)
			}
			fmt.Printf("- **URL**: %s\n", issue.IssueDetailFields.Url)

			// Project and Cycle Info
			if issue.IssueDetailFields.Project != nil {
				fmt.Printf("\n## Project\n")
				fmt.Printf("- **Name**: %s\n", issue.IssueDetailFields.Project.Name)
				fmt.Printf("- **State**: %s\n", issue.IssueDetailFields.Project.State)
				fmt.Printf("- **Progress**: %.0f%%\n", issue.IssueDetailFields.Project.Progress*100)
				if issue.IssueDetailFields.Project.Health != nil {
					fmt.Printf("- **Health**: %s\n", *issue.IssueDetailFields.Project.Health)
				}
				if issue.IssueDetailFields.Project.Description != "" {
					fmt.Printf("- **Description**: %s\n", issue.IssueDetailFields.Project.Description)
				}
			}

			if issue.IssueDetailFields.Cycle != nil {
				fmt.Printf("\n## Cycle\n")
				fmt.Printf("- **Name**: %s (#%.0f)\n", issue.IssueDetailFields.Cycle.Name, issue.IssueDetailFields.Cycle.Number)
				if issue.IssueDetailFields.Cycle.Description != nil && *issue.IssueDetailFields.Cycle.Description != "" {
					fmt.Printf("- **Description**: %s\n", *issue.IssueDetailFields.Cycle.Description)
				}
				fmt.Printf("- **Period**: %s to %s\n", issue.IssueDetailFields.Cycle.StartsAt, issue.IssueDetailFields.Cycle.EndsAt)
				fmt.Printf("- **Progress**: %.0f%%\n", issue.IssueDetailFields.Cycle.Progress*100)
				if issue.IssueDetailFields.Cycle.CompletedAt != nil {
					fmt.Printf("- **Completed**: %s\n", issue.IssueDetailFields.Cycle.CompletedAt.Format("2006-01-02"))
				}
			}

			// Labels
			if issue.IssueDetailFields.Labels != nil && len(issue.IssueDetailFields.Labels.Nodes) > 0 {
				fmt.Printf("\n## Labels\n")
				for _, label := range issue.IssueDetailFields.Labels.Nodes {
					fmt.Printf("- %s", label.Name)
					if label.Description != nil && *label.Description != "" {
						fmt.Printf(" - %s", *label.Description)
					}
					fmt.Println()
				}
			}

			// Subscribers
			if issue.IssueDetailFields.Subscribers != nil && len(issue.IssueDetailFields.Subscribers.Nodes) > 0 {
				fmt.Printf("\n## Subscribers\n")
				for _, subscriber := range issue.IssueDetailFields.Subscribers.Nodes {
					fmt.Printf("- %s (%s)\n", subscriber.Name, subscriber.Email)
				}
			}

			// Relations
			if issue.IssueDetailFields.Relations != nil && len(issue.IssueDetailFields.Relations.Nodes) > 0 {
				fmt.Printf("\n## Related Issues\n")
				for _, relation := range issue.IssueDetailFields.Relations.Nodes {
					if relation.RelatedIssue != nil {
						relationType := relation.Type
						switch relationType {
						case "blocks":
							relationType = "Blocks"
						case "blocked":
							relationType = "Blocked by"
						case "related":
							relationType = "Related to"
						case "duplicate":
							relationType = "Duplicate of"
						}
						fmt.Printf("- %s: %s - %s", relationType, relation.RelatedIssue.Identifier, relation.RelatedIssue.Title)
						if relation.RelatedIssue.State != nil {
							fmt.Printf(" [%s]", relation.RelatedIssue.State.Name)
						}
						fmt.Println()
					}
				}
			}

			// Reactions
			if len(issue.IssueDetailFields.Reactions) > 0 {
				fmt.Printf("\n## Reactions\n")
				reactionMap := make(map[string][]string)
				for _, reaction := range issue.IssueDetailFields.Reactions {
					reactionMap[reaction.Emoji] = append(reactionMap[reaction.Emoji], reaction.User.Name)
				}
				for emoji, users := range reactionMap {
					fmt.Printf("- %s: %s\n", emoji, strings.Join(users, ", "))
				}
			}

			// Show parent issue if this is a sub-issue
			if issue.IssueDetailFields.Parent != nil {
				fmt.Printf("\n## Parent Issue\n")
				fmt.Printf("- %s: %s\n", issue.IssueDetailFields.Parent.Identifier, issue.IssueDetailFields.Parent.Title)
			}

			// Show sub-issues if any
			if issue.IssueDetailFields.Children != nil && len(issue.IssueDetailFields.Children.Nodes) > 0 {
				fmt.Printf("\n## Sub-issues\n")
				for _, child := range issue.IssueDetailFields.Children.Nodes {
					stateStr := ""
					if child.State != nil {
						switch child.State.Type {
						case "completed", "done":
							stateStr = "[x]"
						case "started", "in_progress":
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
					if child.Assignee != nil {
						assignee = child.Assignee.Name
					}

					fmt.Printf("- %s %s: %s (%s)\n", stateStr, child.Identifier, child.Title, assignee)
				}
			}

			// Show attachments if any
			if issue.IssueDetailFields.Attachments != nil && len(issue.IssueDetailFields.Attachments.Nodes) > 0 {
				fmt.Printf("\n## Attachments\n")
				for _, attachment := range issue.IssueDetailFields.Attachments.Nodes {
					fmt.Printf("- [%s](%s)\n", attachment.Title, attachment.Url)
				}
			}

			// Show recent comments if any
			if issue.IssueDetailFields.Comments != nil && len(issue.IssueDetailFields.Comments.Nodes) > 0 {
				fmt.Printf("\n## Recent Comments\n")
				for _, comment := range issue.IssueDetailFields.Comments.Nodes {
					userName := "Unknown"
					if comment.User != nil {
						userName = comment.User.Name
					}
					fmt.Printf("\n### %s - %s\n", userName, comment.CreatedAt.Format("2006-01-02 15:04"))
					if comment.EditedAt != nil {
						fmt.Printf("*(edited %s)*\n", comment.EditedAt.Format("2006-01-02 15:04"))
					}
					fmt.Printf("%s\n", comment.Body)
					if comment.Children != nil && len(comment.Children.Nodes) > 0 {
						for _, reply := range comment.Children.Nodes {
							replyUserName := "Unknown"
							if reply.User != nil {
								replyUserName = reply.User.Name
							}
							fmt.Printf("\n  **Reply from %s**: %s\n", replyUserName, reply.Body)
						}
					}
				}
				fmt.Printf("\n> Use `lincli comment list %s` to see all comments\n", issue.IssueDetailFields.Identifier)
			}

			// Show history
			if issue.IssueDetailFields.History != nil && len(issue.IssueDetailFields.History.Nodes) > 0 {
				fmt.Printf("\n## Recent History\n")
				for _, entry := range issue.IssueDetailFields.History.Nodes {
					fmt.Printf("\n- **%s** by %s", entry.CreatedAt.Format("2006-01-02 15:04"), entry.Actor.Name)
					changes := []string{}

					if entry.FromState != nil && entry.ToState != nil {
						changes = append(changes, fmt.Sprintf("State: %s → %s", entry.FromState.Name, entry.ToState.Name))
					}
					if entry.FromAssignee != nil && entry.ToAssignee != nil {
						changes = append(changes, fmt.Sprintf("Assignee: %s → %s", entry.FromAssignee.Name, entry.ToAssignee.Name))
					} else if entry.FromAssignee != nil && entry.ToAssignee == nil {
						changes = append(changes, fmt.Sprintf("Unassigned from %s", entry.FromAssignee.Name))
					} else if entry.FromAssignee == nil && entry.ToAssignee != nil {
						changes = append(changes, fmt.Sprintf("Assigned to %s", entry.ToAssignee.Name))
					}
					if entry.FromPriority != nil && entry.ToPriority != nil {
						changes = append(changes, fmt.Sprintf("Priority: %s → %s", priorityToString(int(*entry.FromPriority)), priorityToString(int(*entry.ToPriority))))
					}
					if entry.FromTitle != nil && entry.ToTitle != nil {
						changes = append(changes, fmt.Sprintf("Title: \"%s\" → \"%s\"", *entry.FromTitle, *entry.ToTitle))
					}
					if entry.FromCycle != nil && entry.ToCycle != nil {
						changes = append(changes, fmt.Sprintf("Cycle: %s → %s", entry.FromCycle.Name, entry.ToCycle.Name))
					}
					if entry.FromProject != nil && entry.ToProject != nil {
						changes = append(changes, fmt.Sprintf("Project: %s → %s", entry.FromProject.Name, entry.ToProject.Name))
					}
					if len(entry.AddedLabelIds) > 0 {
						changes = append(changes, fmt.Sprintf("Added %d label(s)", len(entry.AddedLabelIds)))
					}
					if len(entry.RemovedLabelIds) > 0 {
						changes = append(changes, fmt.Sprintf("Removed %d label(s)", len(entry.RemovedLabelIds)))
					}

					if len(changes) > 0 {
						fmt.Printf("\n  - %s", strings.Join(changes, "\n  - "))
					}
					fmt.Println()
				}
			}

			return
		}

		// Rich display
		fmt.Printf("%s %s\n",
			color.New(color.FgCyan, color.Bold).Sprint(issue.IssueDetailFields.Identifier),
			color.New(color.FgWhite, color.Bold).Sprint(issue.IssueDetailFields.Title))

		if issue.IssueDetailFields.Description != nil && *issue.IssueDetailFields.Description != "" {
			fmt.Printf("\n%s\n", *issue.IssueDetailFields.Description)
		}

		fmt.Printf("\n%s\n", color.New(color.FgYellow).Sprint("Details:"))

		if issue.IssueDetailFields.State != nil {
			stateStr := issue.IssueDetailFields.State.Name
			if issue.IssueDetailFields.State.Type == "completed" && issue.IssueDetailFields.CompletedAt != nil {
				stateStr += fmt.Sprintf(" (%s)", issue.IssueDetailFields.CompletedAt.Format("2006-01-02"))
			}
			fmt.Printf("State: %s\n",
				color.New(color.FgGreen).Sprint(stateStr))
		}

		if issue.IssueDetailFields.Assignee != nil {
			fmt.Printf("Assignee: %s\n",
				color.New(color.FgCyan).Sprint(issue.IssueDetailFields.Assignee.Name))
		} else {
			fmt.Printf("Assignee: %s\n",
				color.New(color.FgRed).Sprint("Unassigned"))
		}

		if issue.IssueDetailFields.Team != nil {
			fmt.Printf("Team: %s\n",
				color.New(color.FgMagenta).Sprint(issue.IssueDetailFields.Team.Name))
		}

		fmt.Printf("Priority: %s\n", priorityToString(int(issue.IssueDetailFields.Priority)))

		// Show project and cycle info
		if issue.IssueDetailFields.Project != nil {
			fmt.Printf("Project: %s (%s)\n",
				color.New(color.FgBlue).Sprint(issue.IssueDetailFields.Project.Name),
				color.New(color.FgWhite, color.Faint).Sprintf("%.0f%%", issue.IssueDetailFields.Project.Progress*100))
		}

		if issue.IssueDetailFields.Cycle != nil {
			fmt.Printf("Cycle: %s\n",
				color.New(color.FgMagenta).Sprint(issue.IssueDetailFields.Cycle.Name))
		}

		fmt.Printf("Created: %s\n", issue.IssueDetailFields.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated: %s\n", issue.IssueDetailFields.UpdatedAt.Format("2006-01-02 15:04:05"))

		if issue.IssueDetailFields.DueDate != nil && *issue.IssueDetailFields.DueDate != "" {
			fmt.Printf("Due Date: %s\n",
				color.New(color.FgYellow).Sprint(*issue.IssueDetailFields.DueDate))
		}

		if issue.IssueDetailFields.SnoozedUntilAt != nil {
			fmt.Printf("Snoozed Until: %s\n",
				color.New(color.FgYellow).Sprint(issue.IssueDetailFields.SnoozedUntilAt.Format("2006-01-02 15:04:05")))
		}

		// Show git branch if available
		if issue.IssueDetailFields.BranchName != "" {
			fmt.Printf("Git Branch: %s\n",
				color.New(color.FgGreen).Sprint(issue.IssueDetailFields.BranchName))
		}

		// Show URL
		if issue.IssueDetailFields.Url != "" {
			fmt.Printf("URL: %s\n",
				color.New(color.FgBlue, color.Underline).Sprint(issue.IssueDetailFields.Url))
		}

		// Show parent issue if this is a sub-issue
		if issue.IssueDetailFields.Parent != nil {
			fmt.Printf("\n%s\n", color.New(color.FgYellow).Sprint("Parent Issue:"))
			fmt.Printf("  %s %s\n",
				color.New(color.FgCyan).Sprint(issue.IssueDetailFields.Parent.Identifier),
				issue.IssueDetailFields.Parent.Title)
		}

		// Show sub-issues if any
		if issue.IssueDetailFields.Children != nil && len(issue.IssueDetailFields.Children.Nodes) > 0 {
			fmt.Printf("\n%s\n", color.New(color.FgYellow).Sprint("Sub-issues:"))
			for _, child := range issue.IssueDetailFields.Children.Nodes {
				stateIcon := "○"
				if child.State != nil {
					switch child.State.Type {
					case "completed", "done":
						stateIcon = color.New(color.FgGreen).Sprint("✓")
					case "started", "in_progress":
						stateIcon = color.New(color.FgBlue).Sprint("◐")
					case "canceled":
						stateIcon = color.New(color.FgRed).Sprint("✗")
					}
				}

				assignee := "Unassigned"
				if child.Assignee != nil {
					assignee = child.Assignee.Name
				}

				fmt.Printf("  %s %s %s (%s)\n",
					stateIcon,
					color.New(color.FgCyan).Sprint(child.Identifier),
					child.Title,
					color.New(color.FgWhite, color.Faint).Sprint(assignee))
			}
		}

		// Show attachments if any
		if issue.IssueDetailFields.Attachments != nil && len(issue.IssueDetailFields.Attachments.Nodes) > 0 {
			fmt.Printf("\n%s\n", color.New(color.FgYellow).Sprint("Attachments:"))
			for _, attachment := range issue.IssueDetailFields.Attachments.Nodes {
				fmt.Printf("  📎 %s - %s\n",
					attachment.Title,
					color.New(color.FgBlue, color.Underline).Sprint(attachment.Url))
			}
		}

		// Show recent comments if any
		if issue.IssueDetailFields.Comments != nil && len(issue.IssueDetailFields.Comments.Nodes) > 0 {
			fmt.Printf("\n%s\n", color.New(color.FgYellow).Sprint("Recent Comments:"))
			for _, comment := range issue.IssueDetailFields.Comments.Nodes {
				userName := "Unknown"
				if comment.User != nil {
					userName = comment.User.Name
				}
				fmt.Printf("  💬 %s - %s\n",
					color.New(color.FgCyan).Sprint(userName),
					color.New(color.FgWhite, color.Faint).Sprint(comment.CreatedAt.Format("2006-01-02 15:04")))
				// Show first line of comment
				lines := strings.Split(comment.Body, "\n")
				if len(lines) > 0 && lines[0] != "" {
					preview := lines[0]
					if len(preview) > 60 {
						preview = preview[:57] + "..."
					}
					fmt.Printf("     %s\n", preview)
				}
			}
			fmt.Printf("\n  %s Use 'lincli comment list %s' to see all comments\n",
				color.New(color.FgWhite, color.Faint).Sprint("→"),
				issue.IssueDetailFields.Identifier)
		}
	},
}


func priorityToString(priority int) string {
	switch priority {
	case 0:
		return "None"
	case 1:
		return "Urgent"
	case 2:
		return "High"
	case 3:
		return "Normal"
	case 4:
		return "Low"
	default:
		return "Unknown"
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

var issueAssignCmd = &cobra.Command{
	Use:   "assign [issue-id]",
	Short: "Assign issue to yourself",
	Long:  `Assign an issue to yourself.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error("Not authenticated. Run 'lincli auth' first.", plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)

		// Get current user
		viewerResp, err := api.GetViewer(context.Background(), client)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get current user: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		viewerID := viewerResp.Viewer.UserDetailFields.Id

		// Update issue with assignee
		input := api.IssueUpdateInput{
			AssigneeId: &viewerID,
		}

		updateResp, err := api.UpdateIssue(context.Background(), client, args[0], &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to assign issue: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		issue := updateResp.IssueUpdate.Issue

		if jsonOut {
			output.JSON(issue)
		} else if plaintext {
			fmt.Printf("Assigned issue %s to %s\n",
				issue.IssueListFields.Identifier,
				viewerResp.Viewer.UserDetailFields.Name)
		} else {
			fmt.Printf("%s Assigned issue %s to you\n",
				color.New(color.FgGreen).Sprint("✓"),
				color.New(color.FgCyan, color.Bold).Sprint(issue.IssueListFields.Identifier))
		}
	},
}

var issueCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a new issue",
	Long:    `Create a new issue in Linear.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error("Not authenticated. Run 'lincli auth' first.", plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)

		// Get flags
		title, _ := cmd.Flags().GetString("title")
		description, _ := cmd.Flags().GetString("description")
		teamKey, _ := cmd.Flags().GetString("team")
		priority, _ := cmd.Flags().GetInt("priority")
		assignToMe, _ := cmd.Flags().GetBool("assign-me")

		if title == "" {
			output.Error("Title is required (--title)", plaintext, jsonOut)
			os.Exit(1)
		}

		if teamKey == "" {
			output.Error("Team is required (--team)", plaintext, jsonOut)
			os.Exit(1)
		}

		// Get team ID from key
		team, err := client.GetTeam(context.Background(), teamKey)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to find team '%s': %v", teamKey, err), plaintext, jsonOut)
			os.Exit(1)
		}

		// Build input
		input := map[string]interface{}{
			"title":  title,
			"teamId": team.ID,
		}

		if description != "" {
			input["description"] = description
		}

		if priority >= 0 && priority <= 4 {
			input["priority"] = priority
		}

		if assignToMe {
			viewer, err := client.GetViewer(context.Background())
			if err != nil {
				output.Error(fmt.Sprintf("Failed to get current user: %v", err), plaintext, jsonOut)
				os.Exit(1)
			}
			input["assigneeId"] = viewer.ID
		}

		// Create issue
		issue, err := client.CreateIssue(context.Background(), input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create issue: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(issue)
		} else if plaintext {
			fmt.Printf("Created issue %s: %s\n", issue.Identifier, issue.Title)
		} else {
			fmt.Printf("%s Created issue %s: %s\n",
				color.New(color.FgGreen).Sprint("✓"),
				color.New(color.FgCyan, color.Bold).Sprint(issue.Identifier),
				issue.Title)
			if issue.Assignee != nil {
				fmt.Printf("  Assigned to: %s\n", color.New(color.FgCyan).Sprint(issue.Assignee.Name))
			}
		}
	},
}

var issueUpdateCmd = &cobra.Command{
	Use:   "update [issue-id]",
	Short: "Update an issue",
	Long: `Update various fields of an issue.

Examples:
  lincli issue update LIN-123 --title "New title"
  lincli issue update LIN-123 --description "Updated description"
  lincli issue update LIN-123 --assignee john.doe@company.com
  lincli issue update LIN-123 --state "In Progress"
  lincli issue update LIN-123 --priority 1
  lincli issue update LIN-123 --due-date "2024-12-31"
  lincli issue update LIN-123 --title "New title" --assignee me --priority 2`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error("Not authenticated. Run 'lincli auth' first.", plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)

		// Build update input
		input := make(map[string]interface{})

		// Handle title update
		if cmd.Flags().Changed("title") {
			title, _ := cmd.Flags().GetString("title")
			input["title"] = title
		}

		// Handle description update
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input["description"] = description
		}

		// Handle assignee update
		if cmd.Flags().Changed("assignee") {
			assignee, _ := cmd.Flags().GetString("assignee")
			switch assignee {
			case "me":
				// Get current user
				viewer, err := client.GetViewer(context.Background())
				if err != nil {
					output.Error(fmt.Sprintf("Failed to get current user: %v", err), plaintext, jsonOut)
					os.Exit(1)
				}
				input["assigneeId"] = viewer.ID
			case "unassigned", "":
				input["assigneeId"] = nil
			default:
				// Look up user by email
				users, err := client.GetUsers(context.Background(), 100, "", "")
				if err != nil {
					output.Error(fmt.Sprintf("Failed to get users: %v", err), plaintext, jsonOut)
					os.Exit(1)
				}

				var foundUser *api.User
				for _, user := range users.Nodes {
					if user.Email == assignee || user.Name == assignee {
						foundUser = &user
						break
					}
				}

				if foundUser == nil {
					output.Error(fmt.Sprintf("User not found: %s", assignee), plaintext, jsonOut)
					os.Exit(1)
				}

				input["assigneeId"] = foundUser.ID
			}
		}

		// Handle state update
		if cmd.Flags().Changed("state") {
			stateName, _ := cmd.Flags().GetString("state")

			// First, get the issue to know which team it belongs to
			issue, err := client.GetIssue(context.Background(), args[0])
			if err != nil {
				output.Error(fmt.Sprintf("Failed to get issue: %v", err), plaintext, jsonOut)
				os.Exit(1)
			}

			// States are now embedded in issue.Team.States (no separate API call)
			states := issue.Team.States

			// Find the state by name (case-insensitive)
			var stateID string
			for _, state := range states {
				if strings.EqualFold(state.Name, stateName) {
					stateID = state.ID
					break
				}
			}

			if stateID == "" {
				// Show available states
				var stateNames []string
				for _, state := range states {
					stateNames = append(stateNames, state.Name)
				}
				output.Error(fmt.Sprintf("State '%s' not found. Available states: %s", stateName, strings.Join(stateNames, ", ")), plaintext, jsonOut)
				os.Exit(1)
			}

			input["stateId"] = stateID
		}

		// Handle priority update
		if cmd.Flags().Changed("priority") {
			priority, _ := cmd.Flags().GetInt("priority")
			input["priority"] = priority
		}

		// Handle due date update
		if cmd.Flags().Changed("due-date") {
			dueDate, _ := cmd.Flags().GetString("due-date")
			if dueDate == "" {
				input["dueDate"] = nil
			} else {
				input["dueDate"] = dueDate
			}
		}

		// Check if any updates were specified
		if len(input) == 0 {
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		// Update the issue
		issue, err := client.UpdateIssue(context.Background(), args[0], input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update issue: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(issue)
		} else if plaintext {
			fmt.Printf("Updated issue %s\n", issue.Identifier)
		} else {
			output.Success(fmt.Sprintf("Updated issue %s", issue.Identifier), plaintext, jsonOut)
		}
	},
}

func init() {
	rootCmd.AddCommand(issueCmd)
	issueCmd.AddCommand(issueListCmd)
	issueCmd.AddCommand(issueSearchCmd)
	issueCmd.AddCommand(issueGetCmd)
	issueCmd.AddCommand(issueAssignCmd)
	issueCmd.AddCommand(issueCreateCmd)
	issueCmd.AddCommand(issueUpdateCmd)

	// Issue list flags
	issueListCmd.Flags().StringP("assignee", "a", "", "Filter by assignee (email or 'me')")
	issueListCmd.Flags().StringP("state", "s", "", "Filter by state name")
	issueListCmd.Flags().StringP("team", "t", "", "Filter by team key")
	issueListCmd.Flags().IntP("priority", "r", -1, "Filter by priority (0=None, 1=Urgent, 2=High, 3=Normal, 4=Low)")
	issueListCmd.Flags().IntP("limit", "l", 50, "Maximum number of issues to fetch")
	issueListCmd.Flags().BoolP("include-completed", "c", false, "Include completed and canceled issues")
	issueListCmd.Flags().StringP("sort", "o", "linear", "Sort order: linear (default), created, updated")
	issueListCmd.Flags().StringP("newer-than", "n", "", "Show issues created after this time (default: 6_months_ago, use 'all_time' for no filter)")

	// Issue search flags
	issueSearchCmd.Flags().StringP("assignee", "a", "", "Filter by assignee (email or 'me')")
	issueSearchCmd.Flags().StringP("state", "s", "", "Filter by state name")
	issueSearchCmd.Flags().StringP("team", "t", "", "Filter by team key")
	issueSearchCmd.Flags().IntP("priority", "r", -1, "Filter by priority (0=None, 1=Urgent, 2=High, 3=Normal, 4=Low)")
	issueSearchCmd.Flags().IntP("limit", "l", 50, "Maximum number of issues to fetch")
	issueSearchCmd.Flags().BoolP("include-completed", "c", false, "Include completed and canceled issues")
	issueSearchCmd.Flags().Bool("include-archived", false, "Include archived issues in results")
	issueSearchCmd.Flags().StringP("sort", "o", "linear", "Sort order: linear (default), created, updated")
	issueSearchCmd.Flags().StringP("newer-than", "n", "", "Show issues created after this time (default: 6_months_ago, use 'all_time' for no filter)")

	// Issue create flags
	issueCreateCmd.Flags().StringP("title", "", "", "Issue title (required)")
	issueCreateCmd.Flags().StringP("description", "d", "", "Issue description")
	issueCreateCmd.Flags().StringP("team", "t", "", "Team key (required)")
	issueCreateCmd.Flags().Int("priority", 3, "Priority (0=None, 1=Urgent, 2=High, 3=Normal, 4=Low)")
	issueCreateCmd.Flags().BoolP("assign-me", "m", false, "Assign to yourself")
	_ = issueCreateCmd.MarkFlagRequired("title")
	_ = issueCreateCmd.MarkFlagRequired("team")

	// Issue update flags
	issueUpdateCmd.Flags().String("title", "", "New title for the issue")
	issueUpdateCmd.Flags().StringP("description", "d", "", "New description for the issue")
	issueUpdateCmd.Flags().StringP("assignee", "a", "", "Assignee (email, name, 'me', or 'unassigned')")
	issueUpdateCmd.Flags().StringP("state", "s", "", "State name (e.g., 'Todo', 'In Progress', 'Done')")
	issueUpdateCmd.Flags().Int("priority", -1, "Priority (0=None, 1=Urgent, 2=High, 3=Normal, 4=Low)")
	issueUpdateCmd.Flags().String("due-date", "", "Due date (YYYY-MM-DD format, or empty to remove)")
}

// Filter helper functions for type-safe filter building
func stringEq(val string) *api.StringComparator {
	return &api.StringComparator{Eq: &val}
}

func stringIn(vals []string) *api.StringComparator {
	return &api.StringComparator{In: vals}
}

func stringNin(vals []string) *api.StringComparator {
	return &api.StringComparator{Nin: vals}
}

func boolEq(val bool) *api.BooleanComparator {
	return &api.BooleanComparator{Eq: &val}
}

func numberEq(val float64) *api.NullableNumberComparator {
	return &api.NullableNumberComparator{Eq: &val}
}

func dateGte(val string) *api.DateComparator {
	return &api.DateComparator{Gte: &val}
}

// buildIssueFilterTyped builds a typed IssueFilter from command flags
func buildIssueFilterTyped(cmd *cobra.Command) *api.IssueFilter {
	filter := &api.IssueFilter{}

	// Assignee filter
	if assignee, _ := cmd.Flags().GetString("assignee"); assignee != "" {
		if assignee == "me" {
			filter.Assignee = &api.NullableUserFilter{
				IsMe: boolEq(true),
			}
		} else {
			filter.Assignee = &api.NullableUserFilter{
				Email: stringEq(assignee),
			}
		}
	}

	// State filter
	state, _ := cmd.Flags().GetString("state")
	if state != "" {
		filter.State = &api.WorkflowStateFilter{
			Name: stringEq(state),
		}
	} else {
		// Exclude completed/canceled unless explicitly included
		includeCompleted, _ := cmd.Flags().GetBool("include-completed")
		if !includeCompleted {
			filter.State = &api.WorkflowStateFilter{
				Type: stringNin([]string{"completed", "canceled"}),
			}
		}
	}

	// Team filter
	if team, _ := cmd.Flags().GetString("team"); team != "" {
		filter.Team = &api.TeamFilter{
			Key: stringEq(team),
		}
	}

	// Priority filter
	if priority, _ := cmd.Flags().GetInt("priority"); priority != -1 {
		filter.Priority = numberEq(float64(priority))
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
		filter.CreatedAt = dateGte(createdAt)
	}

	return filter
}
