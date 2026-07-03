package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// commentCmd represents the comment command
var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Manage issue comments",
	Long: `Manage comments on Linear issues including listing and creating comments.

Examples:
  lincli comment list LIN-123        # List comments for an issue
  lincli comment create LIN-123 --body "This is fixed"  # Add a comment`,
}

var commentListCmd = &cobra.Command{
	Use:     "list ISSUE-ID",
	Aliases: []string{"ls"},
	Short:   "List comments for an issue",
	Long:    `List all comments for a specific issue.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		issueID := args[0]

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		// Get limit
		limit, _ := cmd.Flags().GetInt("limit")

		// Get sort option
		sortBy, _ := cmd.Flags().GetString("sort")
		orderBy := ""
		if sortBy != "" {
			switch sortBy {
			case "created", "createdAt":
				orderBy = "createdAt"
			case "updated", "updatedAt":
				orderBy = "updatedAt"
			case "linear":
				// Use empty string for Linear's default sort
				orderBy = ""
			default:
				return fmt.Errorf("Invalid sort option: %s. Valid options are: linear, created, updated", sortBy)
			}
		}

		// Convert limit to pointer
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		// Convert orderBy string to PaginationOrderBy enum
		var orderByEnum *api.PaginationOrderBy
		if orderBy != "" {
			if orderBy == "createdAt" {
				val := api.PaginationOrderByCreatedat
				orderByEnum = &val
			} else if orderBy == "updatedAt" {
				val := api.PaginationOrderByUpdatedat
				orderByEnum = &val
			}
		}

		// Get comments using generated function
		resp, err := api.ListComments(context.Background(), client, issueID, limitPtr, nil, orderByEnum)
		if err != nil {
			return fmt.Errorf("Failed to list comments: %v", err)
		}

		// Check if no comments
		if resp.Issue == nil || resp.Issue.Comments == nil || len(resp.Issue.Comments.Nodes) == 0 {
			if jsonOut {
				output.JSON([]*api.ListCommentsIssueCommentsCommentConnectionNodesComment{})
			} else {
				output.Info(fmt.Sprintf("No comments on issue %s", issueID), plaintext, jsonOut)
			}
			return nil
		}

		comments := resp.Issue.Comments.Nodes

		// Handle output
		if jsonOut {
			output.JSON(comments)
		} else if plaintext {
			for i, comment := range comments {
				if i > 0 {
					fmt.Println("---")
				}
				author := "Unknown"
				if comment.User != nil {
					author = comment.User.Name
				}
				fmt.Printf("Author: %s\n", author)
				fmt.Printf("Date: %s\n", comment.CreatedAt.Format("2006-01-02 15:04:05"))
				fmt.Printf("Comment:\n%s\n", comment.Body)
			}
		} else {
			// Rich display
			fmt.Printf("\n%s Comments on %s (%d)\n\n",
				color.New(color.FgCyan, color.Bold).Sprint("💬"),
				color.New(color.FgCyan).Sprint(issueID),
				len(comments))

			for i, comment := range comments {
				if i > 0 {
					fmt.Println(strings.Repeat("─", 50))
				}

				// Header with author and time
				author := "Unknown"
				if comment.User != nil {
					author = comment.User.Name
				}
				timeAgo := formatTimeAgo(comment.CreatedAt)
				fmt.Printf("%s %s %s\n",
					color.New(color.FgCyan, color.Bold).Sprint(author),
					color.New(color.FgWhite, color.Faint).Sprint("•"),
					color.New(color.FgWhite, color.Faint).Sprint(timeAgo))

				// Comment body
				fmt.Printf("\n%s\n\n", comment.Body)
			}
		}
		return nil
	},
}

var commentCreateCmd = &cobra.Command{
	Use:     "create ISSUE-ID",
	Aliases: []string{"add", "new"},
	Short:   "Create a comment on an issue",
	Long: `Add a new comment to a specific issue.

Use --parent to reply to an existing comment, creating a threaded reply.

Examples:
  lincli comment create LIN-123 --body "This is fixed"
  lincli comment create LIN-123 --body "Agreed" --parent COMMENT-ID`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		issueID := args[0]

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		// Get comment body
		body, _ := cmd.Flags().GetString("body")
		if body == "" {
			return errors.New("Comment body is required (--body)")
		}

		// Create comment input
		input := &api.CommentCreateInput{
			Body:    &body,
			IssueId: &issueID,
		}

		// Optional threading: reply to an existing comment by its ID.
		if parent, _ := cmd.Flags().GetString("parent"); parent != "" {
			input.ParentId = &parent
		}

		// Create comment
		createResp, err := api.CreateComment(context.Background(), client, input)
		if err != nil {
			return fmt.Errorf("Failed to create comment: %v", err)
		}
		comment := createResp.CommentCreate.Comment

		// Handle output
		if jsonOut {
			output.JSON(comment)
		} else if plaintext {
			fmt.Printf("Created comment on %s\n", issueID)
			fmt.Printf("Author: %s\n", comment.CommentFields.User.Name)
			fmt.Printf("Date: %s\n", comment.CommentFields.CreatedAt.Format("2006-01-02 15:04:05"))
		} else {
			fmt.Printf("%s Added comment to %s\n",
				color.New(color.FgGreen).Sprint("✓"),
				color.New(color.FgCyan, color.Bold).Sprint(issueID))
			fmt.Printf("\n%s\n", comment.CommentFields.Body)
		}
		return nil
	},
}

var commentEditCmd = &cobra.Command{
	Use:     "edit <comment-id>",
	Aliases: []string{"update"},
	Short:   "Edit a comment's body",
	Long:    `Edit the body of an existing comment. Identify the comment by its ID.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		body, _ := cmd.Flags().GetString("body")
		if body == "" {
			return errors.New("Comment body is required (--body)")
		}

		input := &api.CommentUpdateInput{Body: &body}
		resp, err := api.UpdateComment(context.Background(), client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to edit comment: %v", err)
		}
		if resp.CommentUpdate == nil || resp.CommentUpdate.Comment == nil {
			return errors.New("Failed to edit comment")
		}

		if jsonOut {
			output.JSON(resp.CommentUpdate.Comment)
		} else {
			output.Success(fmt.Sprintf("Edited comment %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var commentDeleteCmd = &cobra.Command{
	Use:     "delete <comment-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a comment",
	Long: `Delete a comment by its ID. This action executes immediately with no
confirmation prompt, and it is permanent: unlike documents, Linear has no
comment-restore API, so a deleted comment cannot be recovered.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		resp, err := api.DeleteComment(context.Background(), client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete comment: %v", err)
		}
		if resp.CommentDelete == nil || !resp.CommentDelete.Success {
			return errors.New("Failed to delete comment")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.CommentDelete.EntityId})
		} else {
			output.Success(fmt.Sprintf("Deleted comment %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var commentResolveCmd = &cobra.Command{
	Use:   "resolve <comment-id>",
	Short: "Resolve a comment thread",
	Long:  `Resolve a comment thread, marking the root comment as resolved.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommentResolution(cmd, args[0], true)
	},
}

var commentUnresolveCmd = &cobra.Command{
	Use:   "unresolve <comment-id>",
	Short: "Unresolve a comment thread",
	Long:  `Clear the resolved state on a previously resolved comment thread.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommentResolution(cmd, args[0], false)
	},
}

// runCommentResolution handles resolve/unresolve, which share the same shape:
// call the mutation, check success, report.
func runCommentResolution(cmd *cobra.Command, id string, resolve bool) error {
	plaintext := viper.GetBool("plaintext")
	jsonOut := viper.GetBool("json")

	client, err := newGraphQLClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	// verb (infinitive) is used in error messages; pastVerb in the success message.
	verb, pastVerb := "resolve", "Resolved"
	if !resolve {
		verb, pastVerb = "unresolve", "Unresolved"
	}

	var success bool
	if resolve {
		resp, err := api.ResolveComment(ctx, client, id)
		if err != nil {
			return fmt.Errorf("Failed to resolve comment: %v", err)
		}
		success = resp.CommentResolve != nil && resp.CommentResolve.Success
	} else {
		resp, err := api.UnresolveComment(ctx, client, id)
		if err != nil {
			return fmt.Errorf("Failed to unresolve comment: %v", err)
		}
		success = resp.CommentUnresolve != nil && resp.CommentUnresolve.Success
	}

	if !success {
		return fmt.Errorf("Failed to %s comment", verb)
	}

	if jsonOut {
		output.JSON(map[string]interface{}{"success": true, "id": id})
		return nil
	}
	output.Success(fmt.Sprintf("%s comment %s", pastVerb, id), plaintext, jsonOut)
	return nil
}

var commentReactCmd = &cobra.Command{
	Use:   "react <comment-id>",
	Short: "React to a comment with an emoji",
	Long: `Add or remove an emoji reaction on a comment.

Examples:
  lincli comment react COMMENT-ID --emoji 👍
  lincli comment react COMMENT-ID --emoji 👍 --remove`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReaction(cmd, "comment", args[0])
	},
}

// formatTimeAgo formats a time as a human-readable "time ago" string
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 30*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / (24 * 30))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	} else {
		years := int(duration.Hours() / (24 * 365))
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

func init() {
	rootCmd.AddCommand(commentCmd)
	commentCmd.AddCommand(commentListCmd)
	commentCmd.AddCommand(commentCreateCmd)
	commentCmd.AddCommand(commentEditCmd)
	commentCmd.AddCommand(commentDeleteCmd)
	commentCmd.AddCommand(commentResolveCmd)
	commentCmd.AddCommand(commentUnresolveCmd)
	commentCmd.AddCommand(commentReactCmd)

	// List command flags
	commentListCmd.Flags().IntP("limit", "l", 50, "Maximum number of comments to return")
	commentListCmd.Flags().StringP("sort", "o", "linear", "Sort order: linear (default), created, updated")

	// Create command flags
	commentCreateCmd.Flags().StringP("body", "b", "", "Comment body (required)")
	commentCreateCmd.Flags().String("parent", "", "Parent comment ID to reply to (creates a threaded reply)")
	_ = commentCreateCmd.MarkFlagRequired("body")

	// Edit command flags
	commentEditCmd.Flags().StringP("body", "b", "", "New comment body (required)")
	_ = commentEditCmd.MarkFlagRequired("body")

	// React command flags
	addReactionFlags(commentReactCmd)
}
