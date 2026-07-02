package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/shanedolley/lincli/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var documentCmd = &cobra.Command{
	Use:     "document",
	Aliases: []string{"doc"},
	Short:   "Manage Linear documents",
	Long: `Manage Linear documents: workspace docs attached to a project or an initiative.

Examples:
  lincli document list                              # List documents
  lincli document list --project "Q1 Roadmap"       # Documents in a project
  lincli document search "onboarding"               # Full-text search
  lincli document get DOC-ID                         # Show a document with its content
  lincli document create --title "Spec" --project "Q1 Roadmap"
  lincli document update DOC-ID --title "New title"
  lincli document delete DOC-ID
  lincli document unarchive DOC-ID`,
}

var documentListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List documents",
	Long:    `List documents, optionally filtered by project, initiative, or creation time.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		filter := &api.DocumentFilter{}

		if project, _ := cmd.Flags().GetString("project"); project != "" {
			projectID, err := resolveProject(ctx, client, cache, project)
			if err != nil {
				return err
			}
			filter.Project = &api.ProjectFilter{Id: &api.IDComparator{Eq: &projectID}}
		}
		if initiative, _ := cmd.Flags().GetString("initiative"); initiative != "" {
			initiativeID, err := resolveInitiative(ctx, client, cache, initiative)
			if err != nil {
				return err
			}
			filter.Initiative = &api.InitiativeFilter{Id: &api.IDComparator{Eq: &initiativeID}}
		}

		newerThan, _ := cmd.Flags().GetString("newer-than")
		createdAt, err := utils.ParseTimeExpression(newerThan)
		if err != nil {
			return fmt.Errorf("Invalid newer-than value: %v", err)
		}
		if createdAt != "" {
			filter.CreatedAt = &api.DateComparator{Gte: &createdAt}
		}

		var orderBy *api.PaginationOrderBy
		switch sortBy, _ := cmd.Flags().GetString("sort"); sortBy {
		case "", "linear":
		case "created", "createdAt":
			v := api.PaginationOrderByCreatedat
			orderBy = &v
		case "updated", "updatedAt":
			v := api.PaginationOrderByUpdatedat
			orderBy = &v
		default:
			return fmt.Errorf("Invalid sort option: %s. Valid options are: linear, created, updated", sortBy)
		}

		resp, err := api.ListDocuments(ctx, client, filter, limitPtr, nil, orderBy)
		if err != nil {
			return fmt.Errorf("Failed to list documents: %v", err)
		}

		if resp.Documents == nil || len(resp.Documents.Nodes) == 0 {
			output.Info("No documents found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.Documents.Nodes)
			return nil
		}

		if plaintext {
			fmt.Println("# Documents")
			for _, node := range resp.Documents.Nodes {
				f := node.DocumentListFields
				fmt.Printf("## %s\n", documentTitle(f.Title))
				fmt.Printf("- **ID**: %s\n", f.Id)
				fmt.Printf("- **Parent**: %s\n", documentParent(f))
				fmt.Printf("- **URL**: %s\n", f.Url)
				fmt.Println()
			}
			fmt.Printf("\nTotal: %d documents\n", len(resp.Documents.Nodes))
			return nil
		}

		headers := []string{"ID", "Title", "Parent", "Creator", "Updated"}
		rows := make([][]string, len(resp.Documents.Nodes))
		for i, node := range resp.Documents.Nodes {
			f := node.DocumentListFields
			creator := ""
			if f.Creator != nil {
				creator = f.Creator.Name
			}
			rows[i] = []string{
				f.Id,
				truncateString(documentTitle(f.Title), 40),
				truncateString(documentParent(f), 25),
				creator,
				f.UpdatedAt.Format("2006-01-02"),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d documents\n", color.New(color.FgGreen).Sprint("✓"), len(resp.Documents.Nodes))
		}
		return nil
	},
}

var documentSearchCmd = &cobra.Command{
	Use:   "search <term>",
	Short: "Search documents by text",
	Long:  `Full-text search across documents. Results are ranked by relevance.`,
	Args:  cobra.ExactArgs(1),
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

		resp, err := api.SearchDocuments(ctx, client, args[0], limitPtr)
		if err != nil {
			return fmt.Errorf("Failed to search documents: %v", err)
		}

		if resp.SearchDocuments == nil || len(resp.SearchDocuments.Nodes) == 0 {
			output.Info(fmt.Sprintf("No documents matching %q", args[0]), plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.SearchDocuments.Nodes)
			return nil
		}

		headers := []string{"ID", "Title", "URL"}
		rows := make([][]string, len(resp.SearchDocuments.Nodes))
		for i, node := range resp.SearchDocuments.Nodes {
			rows[i] = []string{
				node.Id,
				truncateString(documentTitle(node.Title), 50),
				node.Url,
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d of %.0f matches\n", color.New(color.FgGreen).Sprint("✓"), len(resp.SearchDocuments.Nodes), resp.SearchDocuments.TotalCount)
		}
		return nil
	},
}

var documentGetCmd = &cobra.Command{
	Use:     "get <document-id>",
	Aliases: []string{"show"},
	Short:   "Get a document with its content",
	Long: `Get a document's details, including its markdown content.

Use --history to also show the document's content revision history
(version count and timestamps).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.GetDocument(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to get document: %v", err)
		}
		if resp.Document == nil {
			return fmt.Errorf("Document not found: %s", args[0])
		}
		d := resp.Document.DocumentDetailFields
		f := d.DocumentListFields

		// --history fetches the content revision history via the document's
		// content ID (a separate query keyed on documentContentId, not the
		// document ID).
		showHistory, _ := cmd.Flags().GetBool("history")
		var history *api.DocumentContentHistoryResponse
		if showHistory && d.DocumentContentId != nil {
			history, err = api.DocumentContentHistory(ctx, client, *d.DocumentContentId)
			if err != nil {
				return fmt.Errorf("Failed to load content history: %v", err)
			}
		}

		if jsonOut {
			if showHistory {
				output.JSON(map[string]interface{}{"document": resp.Document, "history": documentHistoryNodes(history)})
			} else {
				output.JSON(resp.Document)
			}
			return nil
		}

		content := ""
		if d.Content != nil {
			content = *d.Content
		}

		if plaintext {
			fmt.Printf("# %s\n\n", documentTitle(f.Title))
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Parent**: %s\n", documentParent(f))
			fmt.Printf("- **URL**: %s\n", f.Url)
			fmt.Println()
			if content != "" {
				fmt.Println(content)
			}
			if showHistory {
				printDocumentHistory(history)
			}
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Document:"), documentTitle(f.Title))
		fmt.Printf("  ID:      %s\n", f.Id)
		fmt.Printf("  Parent:  %s\n", documentParent(f))
		if f.Creator != nil {
			fmt.Printf("  Creator: %s\n", f.Creator.Name)
		}
		fmt.Printf("  URL:     %s\n", f.Url)
		if content != "" {
			fmt.Printf("\n%s\n", content)
		}
		if showHistory {
			printDocumentHistory(history)
		}
		return nil
	},
}

var documentCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a document",
	Long:    `Create a document under exactly one parent: a project or an initiative.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		title, _ := cmd.Flags().GetString("title")
		if title == "" {
			return errors.New("Title is required (--title)")
		}

		project, _ := cmd.Flags().GetString("project")
		initiative, _ := cmd.Flags().GetString("initiative")
		if err := validateDocumentParent(project, initiative); err != nil {
			return err
		}

		input := &api.DocumentCreateInput{Title: title}

		if project != "" {
			projectID, err := resolveProject(ctx, client, cache, project)
			if err != nil {
				return err
			}
			input.ProjectId = &projectID
		}
		if initiative != "" {
			initiativeID, err := resolveInitiative(ctx, client, cache, initiative)
			if err != nil {
				return err
			}
			input.InitiativeId = &initiativeID
		}
		if content, _ := cmd.Flags().GetString("content"); content != "" {
			input.Content = &content
		}
		if icon, _ := cmd.Flags().GetString("icon"); icon != "" {
			input.Icon = &icon
		}
		if c, _ := cmd.Flags().GetString("color"); c != "" {
			input.Color = &c
		}

		resp, err := api.DocumentCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to create document: %v", err)
		}
		if resp.DocumentCreate == nil || !resp.DocumentCreate.Success {
			return errors.New("Failed to create document")
		}

		if jsonOut {
			output.JSON(resp.DocumentCreate.Document)
		} else {
			output.Success(fmt.Sprintf("Created document %s", documentTitle(resp.DocumentCreate.Document.DocumentDetailFields.DocumentListFields.Title)), plaintext, jsonOut)
		}
		return nil
	},
}

var documentUpdateCmd = &cobra.Command{
	Use:   "update <document-id>",
	Short: "Update a document",
	Long:  `Update a document's title, content, icon, or color.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		input := &api.DocumentUpdateInput{}

		if cmd.Flags().Changed("title") {
			title, _ := cmd.Flags().GetString("title")
			input.Title = &title
		}
		if cmd.Flags().Changed("content") {
			content, _ := cmd.Flags().GetString("content")
			input.Content = &content
		}
		if cmd.Flags().Changed("icon") {
			icon, _ := cmd.Flags().GetString("icon")
			input.Icon = &icon
		}
		if cmd.Flags().Changed("color") {
			c, _ := cmd.Flags().GetString("color")
			input.Color = &c
		}

		if input.Title == nil && input.Content == nil && input.Icon == nil && input.Color == nil {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.DocumentUpdate(context.Background(), client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to update document: %v", err)
		}
		if resp.DocumentUpdate == nil || !resp.DocumentUpdate.Success {
			return errors.New("Failed to update document")
		}

		if jsonOut {
			output.JSON(resp.DocumentUpdate.Document)
		} else {
			output.Success(fmt.Sprintf("Updated document %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var documentDeleteCmd = &cobra.Command{
	Use:     "delete <document-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a document",
	Long:    `Delete (trash) a document. Reversible with 'document unarchive'.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		resp, err := api.DocumentDelete(context.Background(), client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete document: %v", err)
		}
		if resp.DocumentDelete == nil || !resp.DocumentDelete.Success {
			return errors.New("Failed to delete document")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted document %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var documentUnarchiveCmd = &cobra.Command{
	Use:     "unarchive <document-id>",
	Aliases: []string{"restore"},
	Short:   "Restore a deleted document",
	Long:    `Restore a previously deleted (trashed) document.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		resp, err := api.DocumentUnarchive(context.Background(), client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to unarchive document: %v", err)
		}
		if resp.DocumentUnarchive == nil || !resp.DocumentUnarchive.Success {
			return errors.New("Failed to unarchive document")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Unarchived document %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// validateDocumentParent enforces that a document is created under exactly one
// parent. lincli scopes creation to a project or an initiative; neither or
// both is an error.
func validateDocumentParent(project, initiative string) error {
	switch {
	case project == "" && initiative == "":
		return fmt.Errorf("exactly one of --project or --initiative is required")
	case project != "" && initiative != "":
		return fmt.Errorf("--project and --initiative are mutually exclusive; specify exactly one")
	default:
		return nil
	}
}

// documentTitle renders a document title, falling back to "(untitled)" for the
// empty title Linear uses for untitled docs.
func documentTitle(title string) string {
	if title == "" {
		return "(untitled)"
	}
	return title
}

// documentParent describes a document's parent (its project or initiative) for
// display, or "-" when it has neither.
func documentParent(f api.DocumentListFields) string {
	if f.Project != nil {
		return "project: " + f.Project.Name
	}
	if f.Initiative != nil {
		return "initiative: " + f.Initiative.Name
	}
	return "-"
}

// documentHistoryNodes returns the content-history entries for JSON output,
// or an empty slice when history is absent.
func documentHistoryNodes(hist *api.DocumentContentHistoryResponse) interface{} {
	if hist == nil || hist.DocumentContentHistory == nil {
		return []interface{}{}
	}
	return hist.DocumentContentHistory.History
}

// printDocumentHistory renders a document's content revision history as a
// version count followed by each revision's timestamp.
func printDocumentHistory(hist *api.DocumentContentHistoryResponse) {
	if hist == nil || hist.DocumentContentHistory == nil {
		fmt.Printf("\nHistory: 0 version(s)\n")
		return
	}
	entries := hist.DocumentContentHistory.History
	fmt.Printf("\nHistory: %d version(s)\n", len(entries))
	for _, e := range entries {
		fmt.Printf("  - %s\n", e.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

func init() {
	rootCmd.AddCommand(documentCmd)
	documentCmd.AddCommand(documentListCmd)
	documentCmd.AddCommand(documentSearchCmd)
	documentCmd.AddCommand(documentGetCmd)
	documentCmd.AddCommand(documentCreateCmd)
	documentCmd.AddCommand(documentUpdateCmd)
	documentCmd.AddCommand(documentDeleteCmd)
	documentCmd.AddCommand(documentUnarchiveCmd)

	documentListCmd.Flags().IntP("limit", "l", 50, "Maximum number of documents to fetch")
	documentListCmd.Flags().String("project", "", "Filter by project name or ID")
	documentListCmd.Flags().StringP("initiative", "i", "", "Filter by initiative name or ID")
	documentListCmd.Flags().StringP("sort", "o", "linear", "Sort order: linear (default), created, updated")
	documentListCmd.Flags().StringP("newer-than", "n", "all_time", "Show documents created after this time (e.g. 3_months_ago; default all_time)")

	documentGetCmd.Flags().Bool("history", false, "Also show the document's content revision history")

	documentSearchCmd.Flags().IntP("limit", "l", 25, "Maximum number of results to fetch")

	documentCreateCmd.Flags().String("title", "", "Document title (required)")
	documentCreateCmd.Flags().String("content", "", "Document content (markdown)")
	documentCreateCmd.Flags().String("project", "", "Parent project name or ID")
	documentCreateCmd.Flags().StringP("initiative", "i", "", "Parent initiative name or ID")
	documentCreateCmd.Flags().String("icon", "", "Document icon (emoji)")
	documentCreateCmd.Flags().String("color", "", "Document icon color as a HEX string (e.g. #EB5757)")

	documentUpdateCmd.Flags().String("title", "", "New document title")
	documentUpdateCmd.Flags().String("content", "", "New document content (markdown)")
	documentUpdateCmd.Flags().String("icon", "", "New document icon (emoji)")
	documentUpdateCmd.Flags().String("color", "", "New document icon color as a HEX string (e.g. #EB5757)")
}
