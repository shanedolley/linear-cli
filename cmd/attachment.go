package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// attachmentCmd represents the attachment command
var attachmentCmd = &cobra.Command{
	Use:   "attachment",
	Short: "Manage issue attachments",
	Long: `Manage attachments on Linear issues.

Supports both file uploads to Linear's storage and URL attachments.
Use subcommands to list, create, update, and delete attachments.`,
}

func init() {
	rootCmd.AddCommand(attachmentCmd)
}

var attachmentListCmd = &cobra.Command{
	Use:   "list <issue-id>",
	Short: "List attachments on an issue",
	Long:  `List all attachments (both files and URLs) on a Linear issue.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		issueID := args[0]

		// Get output flags
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		// Get flags
		limit, _ := cmd.Flags().GetInt("limit")
		sortFlag, _ := cmd.Flags().GetString("sort")

		// Convert sort flag to enum
		var orderByEnum *api.PaginationOrderBy
		if sortFlag != "" {
			switch sortFlag {
			case "linear":
				// Use Linear's default sort order (nil orderBy)
				orderByEnum = nil
			case "created", "createdAt":
				val := api.PaginationOrderByCreatedat
				orderByEnum = &val
			case "updated", "updatedAt":
				val := api.PaginationOrderByUpdatedat
				orderByEnum = &val
			default:
				output.Error(fmt.Sprintf("Invalid sort option: %s. Valid options are: linear, created, updated", sortFlag), plaintext, jsonOut)
				os.Exit(1)
			}
		}

		// Get auth and create client
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		// Convert limit to pointer
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		// Call API
		resp, err := api.ListAttachments(ctx, client, issueID, limitPtr, nil, orderByEnum)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list attachments: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		// Check if issue exists
		if resp.Issue == nil {
			output.Error(fmt.Sprintf("Issue %s not found", issueID), plaintext, jsonOut)
			os.Exit(1)
		}

		attachments := resp.Issue.Attachments.Nodes

		// Render output
		if jsonOut {
			output.JSON(attachments)
			return
		}

		if len(attachments) == 0 {
			fmt.Println("No attachments found")
			return
		}

		if plaintext {
			fmt.Printf("# Attachments for %s\n\n", issueID)
			for _, att := range attachments {
				fmt.Printf("## %s\n", att.Title)
				fmt.Printf("- **ID**: %s\n", att.Id)
				if att.Subtitle != nil {
					fmt.Printf("- **Subtitle**: %s\n", *att.Subtitle)
				}
				fmt.Printf("- **URL**: %s\n", att.Url)
				fmt.Printf("- **Created**: %s\n", att.CreatedAt.Format("2006-01-02"))
				if att.Creator != nil {
					fmt.Printf("- **Creator**: %s\n", att.Creator.Name)
				}
				fmt.Println()
			}
		} else {
			// Table output
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Title", "Subtitle", "Creator", "Created"})
			table.SetBorder(false)
			table.SetAutoWrapText(false)

			for _, att := range attachments {
				subtitle := ""
				if att.Subtitle != nil {
					subtitle = *att.Subtitle
				}
				creator := ""
				if att.Creator != nil {
					creator = att.Creator.Name
				}
				table.Append([]string{
					att.Id,
					att.Title,
					subtitle,
					creator,
					att.CreatedAt.Format("2006-01-02"),
				})
			}
			table.Render()
		}
	},
}

func init() {
	attachmentCmd.AddCommand(attachmentListCmd)
	attachmentListCmd.Flags().IntP("limit", "l", 50, "Maximum number of attachments to return")
	attachmentListCmd.Flags().StringP("sort", "o", "", "Sort order: linear (default), created, updated")
}

var attachmentCreateCmd = &cobra.Command{
	Use:   "create <issue-id>",
	Short: "Create a URL attachment on an issue",
	Long:  `Create an attachment linking to an external URL (e.g., GitHub PR, documentation).`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		issueID := args[0]

		// Get output flags
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		// Get flags
		url, _ := cmd.Flags().GetString("url")
		title, _ := cmd.Flags().GetString("title")
		subtitle, _ := cmd.Flags().GetString("subtitle")
		iconURL, _ := cmd.Flags().GetString("icon-url")
		metadataStr, _ := cmd.Flags().GetString("metadata")

		// Validate required flags
		if url == "" {
			output.Error("--url is required", plaintext, jsonOut)
			os.Exit(1)
		}
		if title == "" {
			output.Error("--title is required", plaintext, jsonOut)
			os.Exit(1)
		}

		// Parse metadata
		var metadata *map[string]interface{}
		if metadataStr != "" {
			var err error
			metadataMap, err := parseMetadata(metadataStr)
			if err != nil {
				output.Error(fmt.Sprintf("Invalid metadata: %v", err), plaintext, jsonOut)
				os.Exit(1)
			}
			metadata = &metadataMap
		}

		// Build input
		input := api.AttachmentCreateInput{
			IssueId: issueID,
			Title:   title,
			Url:     url,
		}
		if subtitle != "" {
			input.Subtitle = &subtitle
		}
		if iconURL != "" {
			input.IconUrl = &iconURL
		}
		if metadata != nil {
			input.Metadata = metadata
		}

		// Get auth and create client
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		// Call API
		resp, err := api.AttachmentCreate(ctx, client, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create attachment: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if !resp.AttachmentCreate.Success {
			output.Error("Failed to create attachment", plaintext, jsonOut)
			os.Exit(1)
		}

		// Render output
		if jsonOut {
			output.JSON(resp.AttachmentCreate.Attachment)
			return
		}

		fmt.Printf("✓ Created attachment: %s\n", resp.AttachmentCreate.Attachment.Title)
		fmt.Printf("  ID: %s\n", resp.AttachmentCreate.Attachment.Id)
		fmt.Printf("  URL: %s\n", resp.AttachmentCreate.Attachment.Url)
	},
}

// parseMetadata parses comma-separated key=value pairs
func parseMetadata(s string) (map[string]interface{}, error) {
	metadata := make(map[string]interface{})
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("metadata must be key=value pairs separated by commas")
		}
		metadata[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return metadata, nil
}

func init() {
	attachmentCmd.AddCommand(attachmentCreateCmd)
	attachmentCreateCmd.Flags().String("url", "", "URL to attach (required)")
	attachmentCreateCmd.Flags().String("title", "", "Attachment title (required)")
	attachmentCreateCmd.Flags().String("subtitle", "", "Attachment subtitle")
	attachmentCreateCmd.Flags().String("icon-url", "", "Custom icon URL")
	attachmentCreateCmd.Flags().String("metadata", "", "Metadata as key=value pairs (comma-separated)")
	attachmentCreateCmd.MarkFlagRequired("url")
	attachmentCreateCmd.MarkFlagRequired("title")
}
