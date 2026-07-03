package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

// issueCmd represents the issue command
var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Manage Linear issues",
	Long: `Create, list, update, and manage Linear issues.

Beyond the basics, issues support lifecycle actions (archive, unarchive,
delete, subscribe, unsubscribe, reminder, share, unshare), linking, and bulk
operations (batch-create, batch-update).

Examples:
  lincli issue list --assignee me --state "In Progress"
  lincli issue list --include-completed        # Include completed issues
  lincli issue search "login bug" --team ENG
  lincli issue get LIN-123
  lincli issue create --title "Bug fix" --team ENG --label Bug --estimate 3
  lincli issue update LIN-123 --state Done --cycle 12
  lincli issue archive LIN-123
  lincli issue subscribe LIN-123
  lincli issue reminder LIN-123 --at 2026-08-01
  lincli issue share LIN-123 --user teammate@example.com
  lincli issue batch-create --file issues.csv
  lincli issue batch-update ENG-1 ENG-2 --state Done`,
}

var issueListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List issues",
	Long:    `List Linear issues with optional filtering.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		// Build typed filter from flags
		filterTyped, err := buildIssueFilterTyped(cmd)
		if err != nil {
			return err
		}

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
				return fmt.Errorf("Invalid sort option: %s. Valid options are: linear, created, updated", sortBy)
			}
		}

		// Convert limit to pointer
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		resp, err := api.ListIssues(context.Background(), client, filterTyped, limitPtr, nil, orderByEnum)
		if err != nil {
			return fmt.Errorf("Failed to fetch issues: %w", err)
		}

		// Check if empty
		if len(resp.Issues.Nodes) == 0 {
			output.Info("No issues found", plaintext, jsonOut)
			return nil
		}

		// JSON output
		if jsonOut {
			output.JSON(resp.Issues.Nodes)
			return nil
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
			return nil
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
		return nil
	},
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
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		query := strings.TrimSpace(strings.Join(args, " "))
		if query == "" {
			return errors.New("Search query is required")
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		// Build typed filter from flags
		filterTyped, err := buildIssueFilterTyped(cmd)
		if err != nil {
			return err
		}

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
				return fmt.Errorf("Invalid sort option: %s. Valid options are: linear, created, updated", sortBy)
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
			return fmt.Errorf("Failed to search issues: %w", err)
		}

		// Check if empty
		if len(resp.SearchIssues.Nodes) == 0 {
			output.Info(fmt.Sprintf("No matches found for %q", query), plaintext, jsonOut)
			return nil
		}

		// JSON output
		if jsonOut {
			output.JSON(resp.SearchIssues.Nodes)
			return nil
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
			return nil
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
		return nil
	},
}

var issueGetCmd = &cobra.Command{
	Use:     "get [issue-id]",
	Aliases: []string{"show"},
	Short:   "Get issue details",
	Long:    `Get detailed information about a specific issue.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		resp, err := api.GetIssue(context.Background(), client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to fetch issue: %w", err)
		}
		if resp.Issue == nil {
			return fmt.Errorf("issue %q not found", args[0])
		}
		issue := resp.Issue

		if jsonOut {
			output.JSON(issue.IssueDetailFields)
			return nil
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
			fmt.Printf("- **Priority**: %s (%d)\n", priorityToString(int(issue.IssueDetailFields.Priority)), int(issue.IssueDetailFields.Priority))
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
				fmt.Printf("- **External Creator**: %s (%s)\n", issue.IssueDetailFields.ExternalUserCreator.Name, derefStr(issue.IssueDetailFields.ExternalUserCreator.Email))
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
				fmt.Printf("- **Name**: %s (#%.0f)\n", derefStr(issue.IssueDetailFields.Cycle.Name), issue.IssueDetailFields.Cycle.Number)
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
				// Sort emoji keys so the output is stable across runs (Go map
				// iteration order is randomized).
				emojis := make([]string, 0, len(reactionMap))
				for emoji := range reactionMap {
					emojis = append(emojis, emoji)
				}
				sort.Strings(emojis)
				for _, emoji := range emojis {
					fmt.Printf("- %s: %s\n", emoji, strings.Join(reactionMap[emoji], ", "))
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
						changes = append(changes, fmt.Sprintf("Cycle: %s → %s", derefStr(entry.FromCycle.Name), derefStr(entry.ToCycle.Name)))
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

			return nil
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
		return nil
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

func buildIssueCreateInput(cmd *cobra.Command, teamID string) api.IssueCreateInput {
	input := api.IssueCreateInput{
		TeamId: teamID, // Required field (no pointer)
	}

	if title, _ := cmd.Flags().GetString("title"); title != "" {
		input.Title = &title
	}

	if description, _ := cmd.Flags().GetString("description"); description != "" {
		input.Description = &description
	}

	if priority, _ := cmd.Flags().GetInt("priority"); priority >= 0 && priority <= 4 {
		input.Priority = &priority
	}

	if cmd.Flags().Changed("estimate") {
		estimate, _ := cmd.Flags().GetInt("estimate")
		input.Estimate = &estimate
	}

	return input
}

func buildIssueUpdateInput(cmd *cobra.Command) api.IssueUpdateInput {
	input := api.IssueUpdateInput{}

	if cmd.Flags().Changed("title") {
		title, _ := cmd.Flags().GetString("title")
		input.Title = &title
	}

	if cmd.Flags().Changed("description") {
		description, _ := cmd.Flags().GetString("description")
		input.Description = &description
	}

	// Priority is *int in IssueUpdateInput (not *float64 like IssueCreateInput)
	if cmd.Flags().Changed("priority") {
		priority, _ := cmd.Flags().GetInt("priority")
		input.Priority = &priority
	}

	if cmd.Flags().Changed("estimate") {
		estimate, _ := cmd.Flags().GetInt("estimate")
		input.Estimate = &estimate
	}

	return input
}

// resolveIssueLabels resolves each --label value to a label ID, returning the
// ID slice for use as IssueCreateInput.LabelIds / IssueUpdateInput.LabelIds.
// A resolution failure returns an error for the caller to surface.
func resolveIssueLabels(ctx context.Context, client graphql.Client, cache *ResolverCache, cmd *cobra.Command) ([]string, error) {
	labels, _ := cmd.Flags().GetStringSlice("label")
	ids := make([]string, 0, len(labels))
	for _, label := range labels {
		id, err := resolveLabel(ctx, client, cache, label)
		if err != nil {
			return nil, fmt.Errorf("Failed to find label '%s': %w", label, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// resolveParentIssueID resolves a parent issue reference (identifier or UUID)
// to its UUID via GetIssue, matching how `issue link` resolves parents.
func resolveParentIssueID(ctx context.Context, client graphql.Client, ref string) (string, error) {
	id, err := resolveIssueID(ctx, client, ref)
	if err != nil {
		return "", fmt.Errorf("Failed to find parent %w", err)
	}
	return id, nil
}

var issueAssignCmd = &cobra.Command{
	Use:   "assign [issue-id]",
	Short: "Assign issue to yourself",
	Long:  `Assign an issue to yourself.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		// Get current user
		viewerResp, err := api.GetViewer(context.Background(), client)
		if err != nil {
			return fmt.Errorf("Failed to get current user: %w", err)
		}
		viewerID := viewerResp.Viewer.UserDetailFields.Id

		// Update issue with assignee
		input := api.IssueUpdateInput{
			AssigneeId: &viewerID,
		}

		updateResp, err := api.UpdateIssue(context.Background(), client, args[0], &input)
		if err != nil {
			return fmt.Errorf("Failed to assign issue: %w", err)
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
		return nil
	},
}

var issueCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a new issue",
	Long: `Create a new issue in Linear. --title and --team are required.

Optional fields resolve human-friendly references to IDs: --assignee (email,
name, or 'me'), --state, --project, --label (repeatable), --cycle (number or
name within the team), --parent (issue identifier), and --milestone (which
requires --project, since a milestone belongs to a project).

Examples:
  lincli issue create --title "Fix login" --team ENG
  lincli issue create --title "Fix login" --team ENG --assignee me --priority 1
  lincli issue create --title "Fix login" --team ENG --label Bug --label Backend --estimate 3
  lincli issue create --title "Sub-task" --team ENG --parent ENG-100
  lincli issue create --title "Roadmap item" --team ENG --project "Q3" --milestone "Beta"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		// Get flags
		title, _ := cmd.Flags().GetString("title")
		teamKey, _ := cmd.Flags().GetString("team")
		assignToMe, _ := cmd.Flags().GetBool("assign-me")

		if title == "" {
			return errors.New("Title is required (--title)")
		}

		if teamKey == "" {
			return errors.New("Team is required (--team)")
		}

		// Get team ID from key
		teamID, err := resolveTeam(ctx, client, cache, teamKey)
		if err != nil {
			return fmt.Errorf("Failed to find team '%s': %w", teamKey, err)
		}

		// Build input
		input := buildIssueCreateInput(cmd, teamID)

		if assignToMe {
			viewerID, err := resolveUser(ctx, client, cache, "me")
			if err != nil {
				return fmt.Errorf("Failed to get current user: %w", err)
			}
			input.AssigneeId = &viewerID
		}

		// --assignee (explicit) takes precedence over --assign-me.
		if cmd.Flags().Changed("assignee") {
			assignee, _ := cmd.Flags().GetString("assignee")
			userID, err := resolveUser(ctx, client, cache, assignee)
			if err != nil {
				return err
			}
			input.AssigneeId = &userID
		}

		if cmd.Flags().Changed("state") {
			stateName, _ := cmd.Flags().GetString("state")
			stateID, err := resolveWorkflowState(ctx, client, cache, teamID, stateName)
			if err != nil {
				return err
			}
			input.StateId = &stateID
		}

		if dueDate, _ := cmd.Flags().GetString("due-date"); dueDate != "" {
			input.DueDate = &dueDate
		}

		if cmd.Flags().Changed("label") {
			ids, err := resolveIssueLabels(ctx, client, cache, cmd)
			if err != nil {
				return err
			}
			input.LabelIds = ids
		}

		if cmd.Flags().Changed("cycle") {
			cycle, _ := cmd.Flags().GetString("cycle")
			cycleID, err := resolveCycle(ctx, client, cache, teamID, cycle)
			if err != nil {
				return err
			}
			input.CycleId = &cycleID
		}

		if cmd.Flags().Changed("parent") {
			parent, _ := cmd.Flags().GetString("parent")
			parentID, err := resolveParentIssueID(ctx, client, parent)
			if err != nil {
				return err
			}
			input.ParentId = &parentID
		}

		// --project resolves the target project; a milestone (if given) must
		// belong to it, so it is resolved after the project is known.
		if cmd.Flags().Changed("project") {
			projectName, _ := cmd.Flags().GetString("project")
			projectID, err := resolveProject(ctx, client, cache, projectName)
			if err != nil {
				return err
			}
			input.ProjectId = &projectID
		}

		if cmd.Flags().Changed("milestone") {
			if input.ProjectId == nil {
				return errors.New("--milestone requires --project (a milestone belongs to a project)")
			}
			milestone, _ := cmd.Flags().GetString("milestone")
			milestoneID, err := resolveMilestone(ctx, client, cache, *input.ProjectId, milestone)
			if err != nil {
				return err
			}
			input.ProjectMilestoneId = &milestoneID
		}

		// --template applies an issue template's pre-filled attributes. It is
		// resolved by name within the "issue" template type (or passed as an ID).
		if cmd.Flags().Changed("template") {
			template, _ := cmd.Flags().GetString("template")
			templateID, err := resolveTemplate(ctx, client, cache, "issue", template)
			if err != nil {
				return err
			}
			input.TemplateId = &templateID
		}

		// Create issue
		createResp, err := api.CreateIssue(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to create issue: %w", err)
		}
		issue := createResp.IssueCreate.Issue

		if jsonOut {
			output.JSON(issue)
		} else if plaintext {
			fmt.Printf("Created issue %s: %s\n",
				issue.IssueListFields.Identifier,
				issue.IssueListFields.Title)
		} else {
			fmt.Printf("%s Created issue %s: %s\n",
				color.New(color.FgGreen).Sprint("✓"),
				color.New(color.FgCyan, color.Bold).Sprint(issue.IssueListFields.Identifier),
				issue.IssueListFields.Title)
			if issue.IssueListFields.Assignee != nil {
				fmt.Printf("  Assigned to: %s\n", color.New(color.FgCyan).Sprint(issue.IssueListFields.Assignee.Name))
			}
		}
		return nil
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
  lincli issue update LIN-123 --title "New title" --assignee me --priority 2
  lincli issue update LIN-123 --project "Q1 Roadmap"
  lincli issue update LIN-123 --project none
  lincli issue update LIN-123 --label Bug --label Backend --estimate 5
  lincli issue update LIN-123 --cycle 12 --milestone "Beta"
  lincli issue update LIN-123 --parent ENG-100
  lincli issue update LIN-123 --parent none`,
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

		// Build update input using builder function
		input := buildIssueUpdateInput(cmd)

		// Handle assignee update
		if cmd.Flags().Changed("assignee") {
			assignee, _ := cmd.Flags().GetString("assignee")
			switch assignee {
			case "unassigned", "":
				// Clear the assignee. Use the null sentinel, not a bare nil
				// pointer: the client strips nil fields, so a nil pointer here
				// would be a silent no-op (see api.NullSentinel).
				nullVal := api.NullSentinel
				input.AssigneeId = &nullVal
			default:
				// "me" and email/name lookups are both handled by resolveUser.
				userID, err := resolveUser(ctx, client, cache, assignee)
				if err != nil {
					return err
				}
				input.AssigneeId = &userID
			}
		}

		// The issue's own record (team, project, embedded states) is needed to
		// resolve --state, --cycle by number/name, and --milestone by name.
		// Fetch it at most once and reuse it across those handlers.
		var issueDetail *api.IssueDetailFields
		getIssueDetail := func() (*api.IssueDetailFields, error) {
			if issueDetail == nil {
				issueResp, err := api.GetIssue(ctx, client, args[0])
				if err != nil {
					return nil, fmt.Errorf("Failed to get issue: %w", err)
				}
				if issueResp.Issue == nil {
					return nil, fmt.Errorf("issue %q not found", args[0])
				}
				issueDetail = &issueResp.Issue.IssueDetailFields
			}
			return issueDetail, nil
		}

		// Handle state update - uses workflow states embedded in the issue.
		if cmd.Flags().Changed("state") {
			stateName, _ := cmd.Flags().GetString("state")

			detail, err := getIssueDetail()
			if err != nil {
				return err
			}
			teamKey := detail.Team.Key

			// Seed the resolver cache with the states embedded in the issue
			// response so resolveWorkflowState matches against them without
			// making a separate GetTeamStates request.
			embeddedStates := make([]workflowStateInfo, 0, len(detail.Team.States.Nodes))
			for _, state := range detail.Team.States.Nodes {
				embeddedStates = append(embeddedStates, workflowStateInfo{id: state.Id, name: state.Name})
			}
			cache.states[teamKey] = embeddedStates

			stateID, err := resolveWorkflowState(ctx, client, cache, teamKey, stateName)
			if err != nil {
				return err
			}

			input.StateId = &stateID
		}

		// Handle due date update
		if cmd.Flags().Changed("due-date") {
			dueDate, _ := cmd.Flags().GetString("due-date")
			if dueDate == "" {
				var nilDate *string
				input.DueDate = nilDate
			} else {
				input.DueDate = &dueDate
			}
		}

		// Handle project update
		if cmd.Flags().Changed("project") {
			projectName, _ := cmd.Flags().GetString("project")
			switch strings.ToLower(projectName) {
			case "none", "":
				// Remove from project using null sentinel (converted to null by client)
				nullVal := api.NullSentinel
				input.ProjectId = &nullVal
			default:
				projectID, err := resolveProject(ctx, client, cache, projectName)
				if err != nil {
					return err
				}
				input.ProjectId = &projectID
			}
		}

		if cmd.Flags().Changed("label") {
			ids, err := resolveIssueLabels(ctx, client, cache, cmd)
			if err != nil {
				return err
			}
			input.LabelIds = ids
		}

		// --cycle resolves within the issue's team (unless a UUID is given).
		// resolveCycle filters by team ID, so pass the issue's team UUID.
		if cmd.Flags().Changed("cycle") {
			cycle, _ := cmd.Flags().GetString("cycle")
			teamID := ""
			if !isUUID(cycle) {
				detail, err := getIssueDetail()
				if err != nil {
					return err
				}
				teamID = detail.Team.Id
			}
			cycleID, err := resolveCycle(ctx, client, cache, teamID, cycle)
			if err != nil {
				return err
			}
			input.CycleId = &cycleID
		}

		// --milestone resolves within the issue's project (unless a UUID is
		// given). An issue with no project can't resolve a milestone by name.
		if cmd.Flags().Changed("milestone") {
			milestone, _ := cmd.Flags().GetString("milestone")
			if isUUID(milestone) {
				input.ProjectMilestoneId = &milestone
			} else {
				detail, err := getIssueDetail()
				if err != nil {
					return err
				}
				if detail.Project == nil {
					return errors.New("Cannot resolve --milestone by name: the issue is not in a project. Pass a milestone UUID instead.")
				}
				milestoneID, err := resolveMilestone(ctx, client, cache, detail.Project.Id, milestone)
				if err != nil {
					return err
				}
				input.ProjectMilestoneId = &milestoneID
			}
		}

		// --parent sets or clears the parent issue. "none" clears it. Clearing
		// needs the null sentinel: a bare nil pointer is stripped by the client
		// and would be a silent no-op (see api.NullSentinel).
		if cmd.Flags().Changed("parent") {
			parent, _ := cmd.Flags().GetString("parent")
			if strings.EqualFold(parent, "none") || parent == "" {
				nullVal := api.NullSentinel
				input.ParentId = &nullVal
			} else {
				parentID, err := resolveParentIssueID(ctx, client, parent)
				if err != nil {
					return err
				}
				input.ParentId = &parentID
			}
		}

		// Check if any updates were specified (check all pointer fields)
		hasUpdates := input.Title != nil ||
			input.Description != nil ||
			input.Priority != nil ||
			input.Estimate != nil ||
			input.AssigneeId != nil ||
			input.StateId != nil ||
			input.DueDate != nil ||
			input.LabelIds != nil ||
			input.CycleId != nil ||
			input.ProjectMilestoneId != nil ||
			cmd.Flags().Changed("project") ||
			cmd.Flags().Changed("parent")

		if !hasUpdates {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		// Update the issue using generated function
		updateResp, err := api.UpdateIssue(ctx, client, args[0], &input)
		if err != nil {
			return fmt.Errorf("Failed to update issue: %w", err)
		}
		updatedIssue := updateResp.IssueUpdate.Issue

		if jsonOut {
			output.JSON(updatedIssue)
		} else if plaintext {
			fmt.Printf("Updated issue %s\n", updatedIssue.IssueListFields.Identifier)
		} else {
			output.Success(fmt.Sprintf("Updated issue %s", updatedIssue.IssueListFields.Identifier), plaintext, jsonOut)
		}
		return nil
	},
}

var issueArchiveCmd = &cobra.Command{
	Use:   "archive [issue-id]",
	Short: "Archive an issue",
	Long: `Archive an issue. Use --trash to move it to trash (a soft delete with a
30-day grace period) instead of a plain archive.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		var trashPtr *bool
		if trash, _ := cmd.Flags().GetBool("trash"); trash {
			trashPtr = &trash
		}

		resp, err := api.IssueArchive(ctx, client, args[0], trashPtr)
		if err != nil {
			return fmt.Errorf("Failed to archive issue: %w", err)
		}
		if !resp.IssueArchive.Success {
			return errors.New("Failed to archive issue")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Archived issue %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var issueUnarchiveCmd = &cobra.Command{
	Use:   "unarchive [issue-id]",
	Short: "Unarchive an issue",
	Long:  `Restore a previously archived issue.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.IssueUnarchive(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to unarchive issue: %w", err)
		}
		if !resp.IssueUnarchive.Success {
			return errors.New("Failed to unarchive issue")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Unarchived issue %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var issueDeleteCmd = &cobra.Command{
	Use:   "delete [issue-id]",
	Short: "Delete an issue",
	Long: `Delete (trash) an issue. This action executes immediately with no
confirmation prompt. Use --permanent to skip the 30-day grace period (admin
only).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		var permanentPtr *bool
		if permanent, _ := cmd.Flags().GetBool("permanent"); permanent {
			permanentPtr = &permanent
		}

		resp, err := api.IssueDelete(ctx, client, args[0], permanentPtr)
		if err != nil {
			return fmt.Errorf("Failed to delete issue: %w", err)
		}
		if !resp.IssueDelete.Success {
			return errors.New("Failed to delete issue")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted issue %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var issueSubscribeCmd = &cobra.Command{
	Use:   "subscribe [issue-id]",
	Short: "Subscribe a user to an issue",
	Long:  `Subscribe a user to an issue. Defaults to yourself; use --user to subscribe someone else.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIssueSubscription(cmd, args[0], true)
	},
}

var issueUnsubscribeCmd = &cobra.Command{
	Use:   "unsubscribe [issue-id]",
	Short: "Unsubscribe a user from an issue",
	Long:  `Unsubscribe a user from an issue. Defaults to yourself; use --user to unsubscribe someone else.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIssueSubscription(cmd, args[0], false)
	},
}

// runIssueSubscription handles subscribe/unsubscribe, which share the same
// shape. --user is optional; when omitted, Linear defaults to the current user.
func runIssueSubscription(cmd *cobra.Command, issueID string, subscribe bool) error {
	plaintext := viper.GetBool("plaintext")
	jsonOut := viper.GetBool("json")

	client, err := newGraphQLClient()
	if err != nil {
		return err
	}
	ctx := context.Background()
	cache := newResolverCache()

	var userIDPtr *string
	if user, _ := cmd.Flags().GetString("user"); user != "" {
		userID, err := resolveUser(ctx, client, cache, user)
		if err != nil {
			return err
		}
		userIDPtr = &userID
	}

	var success bool
	if subscribe {
		resp, err := api.IssueSubscribe(ctx, client, issueID, userIDPtr)
		if err != nil {
			return fmt.Errorf("Failed to subscribe to issue: %w", err)
		}
		success = resp.IssueSubscribe.Success
	} else {
		resp, err := api.IssueUnsubscribe(ctx, client, issueID, userIDPtr)
		if err != nil {
			return fmt.Errorf("Failed to unsubscribe from issue: %w", err)
		}
		success = resp.IssueUnsubscribe.Success
	}

	verb := "Subscribed to"
	if !subscribe {
		verb = "Unsubscribed from"
	}
	if !success {
		return fmt.Errorf("Failed to change subscription for issue %s", issueID)
	}

	if jsonOut {
		output.JSON(map[string]interface{}{"success": true, "id": issueID})
	} else {
		output.Success(fmt.Sprintf("%s issue %s", verb, issueID), plaintext, jsonOut)
	}
	return nil
}

var issueReminderCmd = &cobra.Command{
	Use:   "reminder [issue-id]",
	Short: "Set a reminder on an issue",
	Long: `Schedule a reminder notification for an issue at a given time.

Examples:
  lincli issue reminder LIN-123 --at 2026-08-01
  lincli issue reminder LIN-123 --at 2026-08-01T09:00:00Z`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		at, _ := cmd.Flags().GetString("at")
		if at == "" {
			return errors.New("Reminder time is required (--at)")
		}
		reminderAt, err := parseDateTimeFlag(at)
		if err != nil {
			return err
		}

		resp, err := api.IssueReminder(ctx, client, args[0], reminderAt)
		if err != nil {
			return fmt.Errorf("Failed to set reminder: %w", err)
		}
		if !resp.IssueReminder.Success {
			return errors.New("Failed to set reminder")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0], "reminderAt": reminderAt.Format(time.RFC3339)})
		} else {
			output.Success(fmt.Sprintf("Set reminder on issue %s for %s", args[0], reminderAt.Format("2006-01-02 15:04")), plaintext, jsonOut)
		}
		return nil
	},
}

var issueShareCmd = &cobra.Command{
	Use:   "share [issue-id]",
	Short: "Share an issue with a user",
	Long:  `Share an issue with a specific user. --user is required.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIssueShare(cmd, args[0], true)
	},
}

var issueUnshareCmd = &cobra.Command{
	Use:   "unshare [issue-id]",
	Short: "Stop sharing an issue with a user",
	Long:  `Stop sharing an issue with a specific user. --user is required.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIssueShare(cmd, args[0], false)
	},
}

var issueReactCmd = &cobra.Command{
	Use:   "react [issue-id]",
	Short: "React to an issue with an emoji",
	Long: `Add or remove an emoji reaction on an issue.

Examples:
  lincli issue react LIN-123 --emoji 👍
  lincli issue react LIN-123 --emoji 👍 --remove`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReaction(cmd, "issue", args[0])
	},
}

// runIssueShare handles share/unshare, which both require a target --user.
func runIssueShare(cmd *cobra.Command, issueID string, share bool) error {
	plaintext := viper.GetBool("plaintext")
	jsonOut := viper.GetBool("json")

	client, err := newGraphQLClient()
	if err != nil {
		return err
	}
	ctx := context.Background()
	cache := newResolverCache()

	user, _ := cmd.Flags().GetString("user")
	if user == "" {
		return errors.New("A target user is required (--user)")
	}
	userID, err := resolveUser(ctx, client, cache, user)
	if err != nil {
		return err
	}

	var success bool
	var shareURL string
	if share {
		resp, err := api.IssueShare(ctx, client, issueID, userID)
		if err != nil {
			return fmt.Errorf("Failed to share issue: %w", err)
		}
		success = resp.IssueShare.Success
		if resp.IssueShare.Issue != nil {
			shareURL = resp.IssueShare.Issue.IssueListFields.Url
		}
	} else {
		resp, err := api.IssueUnshare(ctx, client, issueID, userID)
		if err != nil {
			return fmt.Errorf("Failed to unshare issue: %w", err)
		}
		success = resp.IssueUnshare.Success
	}

	verb := "Shared"
	if !share {
		verb = "Unshared"
	}
	if !success {
		return fmt.Errorf("Failed to change sharing for issue %s", issueID)
	}

	if jsonOut {
		result := map[string]interface{}{"success": true, "id": issueID, "userId": userID}
		if shareURL != "" {
			result["url"] = shareURL
		}
		output.JSON(result)
		return nil
	}

	output.Success(fmt.Sprintf("%s issue %s with %s", verb, issueID, user), plaintext, jsonOut)
	if share && shareURL != "" {
		fmt.Printf("  URL: %s\n", shareURL)
	}
	return nil
}

var issueLinkCmd = &cobra.Command{
	Use:   "link [source-issue] [target-issue]",
	Short: "Link two issues together",
	Long: `Create a relationship between two issues.

Supported relation types:
  blocks      - Source issue blocks target (target cannot proceed until source is done)
  blocked-by  - Source issue is blocked by target (source cannot proceed until target is done)
  related     - Issues are related to each other
  duplicate   - Source issue is a duplicate of target
  parent-of   - Source issue becomes the parent of target (target becomes sub-issue)
  sub-issue-of - Source issue becomes a sub-issue of target (target becomes parent)

Examples:
  lincli issue link TEAM-123 TEAM-456 --type blocks
  lincli issue link TEAM-123 TEAM-456 --type blocked-by
  lincli issue link TEAM-123 TEAM-456 --type related
  lincli issue link TEAM-123 TEAM-456 --type duplicate
  lincli issue link TEAM-123 TEAM-456 --type parent-of
  lincli issue link TEAM-123 TEAM-456 --type sub-issue-of
  lincli issue link TEAM-123 TEAM-456 --type blocks --remove`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		sourceIssue := args[0]
		targetIssue := args[1]

		// Validate not self-referential
		if strings.EqualFold(sourceIssue, targetIssue) {
			return errors.New("Cannot link an issue to itself")
		}

		linkType, _ := cmd.Flags().GetString("type")
		remove, _ := cmd.Flags().GetBool("remove")

		// Validate type
		validTypes := []string{"blocks", "blocked-by", "related", "duplicate", "parent-of", "sub-issue-of"}
		isValidType := false
		for _, t := range validTypes {
			if linkType == t {
				isValidType = true
				break
			}
		}
		if !isValidType {
			return fmt.Errorf("Invalid type '%s'. Valid types: %s", linkType, strings.Join(validTypes, ", "))
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		// Handle parent/child types differently - they use IssueUpdateInput.parentId
		if linkType == "parent-of" || linkType == "sub-issue-of" {
			return handleParentChildLink(ctx, client, sourceIssue, targetIssue, linkType, remove, plaintext, jsonOut)
		}

		// Handle relation types (blocks, blocked-by, related, duplicate)
		return handleRelationLink(ctx, client, sourceIssue, targetIssue, linkType, remove, plaintext, jsonOut)
	},
}

func handleParentChildLink(ctx context.Context, client graphql.Client, sourceIssue, targetIssue, linkType string, remove bool, plaintext, jsonOut bool) error {
	// For parent-of: target's parentId = source's id
	// For sub-issue-of: source's parentId = target's id

	if remove {
		// To remove a parent relationship, set parentId to nil
		var issueToUpdate string
		if linkType == "parent-of" {
			issueToUpdate = targetIssue
		} else {
			issueToUpdate = sourceIssue
		}

		// Clear parentId. Use the null sentinel, not a bare nil pointer: the
		// client strips nil fields, so a nil pointer here would be a silent
		// no-op and the parent link would never actually be removed.
		nullVal := api.NullSentinel
		input := api.IssueUpdateInput{
			ParentId: &nullVal,
		}

		_, err := api.UpdateIssue(ctx, client, issueToUpdate, &input)
		if err != nil {
			return fmt.Errorf("Failed to remove parent relationship: %w", err)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{
				"success": true,
				"action":  "removed",
				"type":    linkType,
				"source":  sourceIssue,
				"target":  targetIssue,
			})
		} else if plaintext {
			fmt.Printf("Removed %s relationship between %s and %s\n", linkType, sourceIssue, targetIssue)
		} else {
			output.Success(fmt.Sprintf("Removed %s relationship between %s and %s", linkType, sourceIssue, targetIssue), plaintext, jsonOut)
		}
		return nil
	}

	// Get the issue IDs
	var parentIssue, childIssue string
	if linkType == "parent-of" {
		parentIssue = sourceIssue
		childIssue = targetIssue
	} else {
		parentIssue = targetIssue
		childIssue = sourceIssue
	}

	// Get parent issue to get its ID
	parentResp, err := api.GetIssue(ctx, client, parentIssue)
	if err != nil {
		return fmt.Errorf("Failed to get parent issue: %w", err)
	}
	if parentResp.Issue == nil {
		return fmt.Errorf("Parent issue %q not found", parentIssue)
	}
	parentID := parentResp.Issue.IssueDetailFields.Id

	// Update child issue with parentId
	input := api.IssueUpdateInput{
		ParentId: &parentID,
	}

	updateResp, err := api.UpdateIssue(ctx, client, childIssue, &input)
	if err != nil {
		return fmt.Errorf("Failed to create parent relationship: %w", err)
	}

	if jsonOut {
		output.JSON(map[string]interface{}{
			"success": true,
			"type":    linkType,
			"parent":  parentIssue,
			"child":   childIssue,
			"childId": updateResp.IssueUpdate.Issue.IssueListFields.Id,
		})
	} else if plaintext {
		fmt.Printf("Linked %s %s %s\n", sourceIssue, linkType, targetIssue)
	} else {
		fmt.Printf("%s Linked %s %s %s\n",
			color.New(color.FgGreen).Sprint("✓"),
			color.New(color.FgCyan).Sprint(sourceIssue),
			linkType,
			color.New(color.FgCyan).Sprint(targetIssue))
	}
	return nil
}

func handleRelationLink(ctx context.Context, client graphql.Client, sourceIssue, targetIssue, linkType string, remove bool, plaintext, jsonOut bool) error {
	// Get issue IDs first
	sourceResp, err := api.GetIssue(ctx, client, sourceIssue)
	if err != nil {
		return fmt.Errorf("Failed to get source issue: %w", err)
	}
	if sourceResp.Issue == nil {
		return fmt.Errorf("Source issue %q not found", sourceIssue)
	}
	sourceID := sourceResp.Issue.IssueDetailFields.Id

	targetResp, err := api.GetIssue(ctx, client, targetIssue)
	if err != nil {
		return fmt.Errorf("Failed to get target issue: %w", err)
	}
	if targetResp.Issue == nil {
		return fmt.Errorf("Target issue %q not found", targetIssue)
	}
	targetID := targetResp.Issue.IssueDetailFields.Id

	if remove {
		// Find and delete the relation
		// Need to find the relation ID from the source issue's relations
		relations := sourceResp.Issue.IssueDetailFields.Relations
		if relations == nil || len(relations.Nodes) == 0 {
			return fmt.Errorf("No relations found on issue %s", sourceIssue)
		}

		// Map CLI type to API type for matching
		var apiType string
		switch linkType {
		case "blocks":
			apiType = "blocks"
		case "blocked-by":
			// For blocked-by, the relation is stored as "blocks" on the OTHER issue
			// We need to check the target issue's relations instead
			targetRelations := targetResp.Issue.IssueDetailFields.Relations
			if targetRelations != nil {
				for _, rel := range targetRelations.Nodes {
					if rel.RelatedIssue != nil && rel.RelatedIssue.Id == sourceID && rel.Type == "blocks" {
						_, err := api.DeleteIssueRelation(ctx, client, rel.Id)
						if err != nil {
							return fmt.Errorf("Failed to delete relation: %w", err)
						}
						if jsonOut {
							output.JSON(map[string]interface{}{
								"success": true,
								"action":  "removed",
								"type":    linkType,
								"source":  sourceIssue,
								"target":  targetIssue,
							})
						} else if plaintext {
							fmt.Printf("Removed %s relationship between %s and %s\n", linkType, sourceIssue, targetIssue)
						} else {
							output.Success(fmt.Sprintf("Removed %s relationship between %s and %s", linkType, sourceIssue, targetIssue), plaintext, jsonOut)
						}
						return nil
					}
				}
			}
			return fmt.Errorf("No %s relation found between %s and %s", linkType, sourceIssue, targetIssue)
		case "related":
			apiType = "related"
		case "duplicate":
			apiType = "duplicate"
		}

		// Find matching relation
		var relationID string
		for _, rel := range relations.Nodes {
			if rel.RelatedIssue != nil && rel.RelatedIssue.Id == targetID && rel.Type == apiType {
				relationID = rel.Id
				break
			}
		}

		if relationID == "" {
			return fmt.Errorf("No %s relation found between %s and %s", linkType, sourceIssue, targetIssue)
		}

		_, err = api.DeleteIssueRelation(ctx, client, relationID)
		if err != nil {
			return fmt.Errorf("Failed to delete relation: %w", err)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{
				"success": true,
				"action":  "removed",
				"type":    linkType,
				"source":  sourceIssue,
				"target":  targetIssue,
			})
		} else if plaintext {
			fmt.Printf("Removed %s relationship between %s and %s\n", linkType, sourceIssue, targetIssue)
		} else {
			output.Success(fmt.Sprintf("Removed %s relationship between %s and %s", linkType, sourceIssue, targetIssue), plaintext, jsonOut)
		}
		return nil
	}

	// Create the relation
	// For blocked-by, swap source and target to create "target blocks source"
	var issueID, relatedIssueID string
	var relationType api.IssueRelationType

	switch linkType {
	case "blocks":
		issueID = sourceID
		relatedIssueID = targetID
		relationType = api.IssueRelationTypeBlocks
	case "blocked-by":
		// Swap: target blocks source
		issueID = targetID
		relatedIssueID = sourceID
		relationType = api.IssueRelationTypeBlocks
	case "related":
		issueID = sourceID
		relatedIssueID = targetID
		relationType = api.IssueRelationTypeRelated
	case "duplicate":
		issueID = sourceID
		relatedIssueID = targetID
		relationType = api.IssueRelationTypeDuplicate
	}

	input := api.IssueRelationCreateInput{
		IssueId:        issueID,
		RelatedIssueId: relatedIssueID,
		Type:           relationType,
	}

	resp, err := api.CreateIssueRelation(ctx, client, &input)
	if err != nil {
		return fmt.Errorf("Failed to create relation: %w", err)
	}

	if jsonOut {
		output.JSON(map[string]interface{}{
			"success":      resp.IssueRelationCreate.Success,
			"type":         linkType,
			"relationId":   resp.IssueRelationCreate.IssueRelation.Id,
			"issue":        sourceIssue,
			"relatedIssue": targetIssue,
		})
	} else if plaintext {
		fmt.Printf("Linked %s %s %s\n", sourceIssue, linkType, targetIssue)
	} else {
		fmt.Printf("%s Linked %s %s %s\n",
			color.New(color.FgGreen).Sprint("✓"),
			color.New(color.FgCyan).Sprint(sourceIssue),
			linkType,
			color.New(color.FgCyan).Sprint(targetIssue))
	}
	return nil
}

func init() {
	rootCmd.AddCommand(issueCmd)
	issueCmd.AddCommand(issueListCmd)
	issueCmd.AddCommand(issueSearchCmd)
	issueCmd.AddCommand(issueGetCmd)
	issueCmd.AddCommand(issueAssignCmd)
	issueCmd.AddCommand(issueCreateCmd)
	issueCmd.AddCommand(issueUpdateCmd)
	issueCmd.AddCommand(issueLinkCmd)
	issueCmd.AddCommand(issueArchiveCmd)
	issueCmd.AddCommand(issueUnarchiveCmd)
	issueCmd.AddCommand(issueDeleteCmd)
	issueCmd.AddCommand(issueSubscribeCmd)
	issueCmd.AddCommand(issueUnsubscribeCmd)
	issueCmd.AddCommand(issueReminderCmd)
	issueCmd.AddCommand(issueShareCmd)
	issueCmd.AddCommand(issueUnshareCmd)
	issueCmd.AddCommand(issueReactCmd)

	// Issue lifecycle flags
	issueArchiveCmd.Flags().Bool("trash", false, "Move the issue to trash (soft delete, 30-day grace) instead of a plain archive")
	issueDeleteCmd.Flags().Bool("permanent", false, "Permanently delete, skipping the 30-day grace period (admin only)")
	issueSubscribeCmd.Flags().StringP("user", "u", "", "User to subscribe (email, name, or 'me'); defaults to you")
	issueUnsubscribeCmd.Flags().StringP("user", "u", "", "User to unsubscribe (email, name, or 'me'); defaults to you")
	issueReminderCmd.Flags().String("at", "", "Reminder time (YYYY-MM-DD or RFC3339, required)")
	issueShareCmd.Flags().StringP("user", "u", "", "User to share the issue with (email, name, or 'me') (required)")
	issueUnshareCmd.Flags().StringP("user", "u", "", "User to stop sharing with (email, name, or 'me') (required)")
	addReactionFlags(issueReactCmd)

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
	issueCreateCmd.Flags().StringP("assignee", "a", "", "Assignee (email, name, or 'me')")
	issueCreateCmd.Flags().StringP("state", "s", "", "State name (e.g., 'Todo', 'In Progress', 'Done')")
	issueCreateCmd.Flags().String("project", "", "Project name or ID")
	issueCreateCmd.Flags().String("due-date", "", "Due date (YYYY-MM-DD)")
	issueCreateCmd.Flags().StringSlice("label", nil, "Label name or ID (repeatable)")
	issueCreateCmd.Flags().String("cycle", "", "Cycle number, name, or ID (within the team)")
	issueCreateCmd.Flags().String("milestone", "", "Project milestone name or ID (requires --project)")
	issueCreateCmd.Flags().Int("estimate", 0, "Estimate (story points)")
	issueCreateCmd.Flags().String("parent", "", "Parent issue identifier or ID (creates a sub-issue)")
	issueCreateCmd.Flags().String("template", "", "Issue template name or ID to apply")
	_ = issueCreateCmd.MarkFlagRequired("title")
	_ = issueCreateCmd.MarkFlagRequired("team")

	// Issue update flags
	issueUpdateCmd.Flags().String("title", "", "New title for the issue")
	issueUpdateCmd.Flags().StringP("description", "d", "", "New description for the issue")
	issueUpdateCmd.Flags().StringP("assignee", "a", "", "Assignee (email, name, 'me', or 'unassigned')")
	issueUpdateCmd.Flags().StringP("state", "s", "", "State name (e.g., 'Todo', 'In Progress', 'Done')")
	issueUpdateCmd.Flags().Int("priority", -1, "Priority (0=None, 1=Urgent, 2=High, 3=Normal, 4=Low)")
	issueUpdateCmd.Flags().String("due-date", "", "Due date (YYYY-MM-DD format, or empty to remove)")
	issueUpdateCmd.Flags().String("project", "", "Project name (or 'none' to remove from project)")
	issueUpdateCmd.Flags().StringSlice("label", nil, "Label name or ID (repeatable; replaces the issue's labels)")
	issueUpdateCmd.Flags().String("cycle", "", "Cycle number, name, or ID (within the issue's team)")
	issueUpdateCmd.Flags().String("milestone", "", "Project milestone name or ID (within the issue's project)")
	issueUpdateCmd.Flags().Int("estimate", 0, "Estimate (story points)")
	issueUpdateCmd.Flags().String("parent", "", "Parent issue identifier or ID (or 'none' to clear)")

	// Issue link flags
	issueLinkCmd.Flags().StringP("type", "t", "", "Relation type: blocks, blocked-by, related, duplicate, parent-of, sub-issue-of (required)")
	issueLinkCmd.Flags().Bool("remove", false, "Remove the relationship instead of creating it")
	_ = issueLinkCmd.MarkFlagRequired("type")
}

// Filter helper functions for type-safe filter building
func stringEq(val string) *api.StringComparator {
	return &api.StringComparator{Eq: &val}
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
func buildIssueFilterTyped(cmd *cobra.Command) (*api.IssueFilter, error) {
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
		return nil, fmt.Errorf("Invalid newer-than value: %w", err)
	}
	if createdAt != "" {
		filter.CreatedAt = dateGte(createdAt)
	}

	return filter, nil
}
