package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
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

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		filter := &api.DocumentFilter{}

		if project, _ := cmd.Flags().GetString("project"); project != "" {
			projectID, err := resolveProject(ctx, client, cache, project)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			filter.Project = &api.ProjectFilter{Id: &api.IDComparator{Eq: &projectID}}
		}
		if initiative, _ := cmd.Flags().GetString("initiative"); initiative != "" {
			initiativeID, err := resolveInitiative(ctx, client, cache, initiative)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			filter.Initiative = &api.InitiativeFilter{Id: &api.IDComparator{Eq: &initiativeID}}
		}

		newerThan, _ := cmd.Flags().GetString("newer-than")
		createdAt, err := utils.ParseTimeExpression(newerThan)
		if err != nil {
			output.Error(fmt.Sprintf("Invalid newer-than value: %v", err), plaintext, jsonOut)
			os.Exit(1)
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
			output.Error(fmt.Sprintf("Invalid sort option: %s. Valid options are: linear, created, updated", sortBy), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.ListDocuments(ctx, client, filter, limitPtr, nil, orderBy)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list documents: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if resp.Documents == nil || len(resp.Documents.Nodes) == 0 {
			output.Info("No documents found", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(resp.Documents.Nodes)
			return
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
			return
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
	},
}

var documentSearchCmd = &cobra.Command{
	Use:   "search <term>",
	Short: "Search documents by text",
	Long:  `Full-text search across documents. Results are ranked by relevance.`,
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

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		resp, err := api.SearchDocuments(ctx, client, args[0], limitPtr)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to search documents: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if resp.SearchDocuments == nil || len(resp.SearchDocuments.Nodes) == 0 {
			output.Info(fmt.Sprintf("No documents matching %q", args[0]), plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(resp.SearchDocuments.Nodes)
			return
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

		resp, err := api.GetDocument(ctx, client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get document: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.Document == nil {
			output.Error(fmt.Sprintf("Document not found: %s", args[0]), plaintext, jsonOut)
			os.Exit(1)
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
				output.Error(fmt.Sprintf("Failed to load content history: %v", err), plaintext, jsonOut)
				os.Exit(1)
			}
		}

		if jsonOut {
			if showHistory {
				output.JSON(map[string]interface{}{"document": resp.Document, "history": documentHistoryNodes(history)})
			} else {
				output.JSON(resp.Document)
			}
			return
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
			return
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
	},
}

var documentCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a document",
	Long:    `Create a document under exactly one parent: a project or an initiative.`,
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

		title, _ := cmd.Flags().GetString("title")
		if title == "" {
			output.Error("Title is required (--title)", plaintext, jsonOut)
			os.Exit(1)
		}

		project, _ := cmd.Flags().GetString("project")
		initiative, _ := cmd.Flags().GetString("initiative")
		if err := validateDocumentParent(project, initiative); err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		input := &api.DocumentCreateInput{Title: title}

		if project != "" {
			projectID, err := resolveProject(ctx, client, cache, project)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.ProjectId = &projectID
		}
		if initiative != "" {
			initiativeID, err := resolveInitiative(ctx, client, cache, initiative)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
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
			output.Error(fmt.Sprintf("Failed to create document: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.DocumentCreate == nil || !resp.DocumentCreate.Success {
			output.Error("Failed to create document", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.DocumentCreate.Document)
		} else {
			output.Success(fmt.Sprintf("Created document %s", documentTitle(resp.DocumentCreate.Document.DocumentDetailFields.DocumentListFields.Title)), plaintext, jsonOut)
		}
	},
}

var documentUpdateCmd = &cobra.Command{
	Use:   "update <document-id>",
	Short: "Update a document",
	Long:  `Update a document's title, content, icon, or color.`,
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
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.DocumentUpdate(context.Background(), client, args[0], input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update document: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.DocumentUpdate == nil || !resp.DocumentUpdate.Success {
			output.Error("Failed to update document", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.DocumentUpdate.Document)
		} else {
			output.Success(fmt.Sprintf("Updated document %s", args[0]), plaintext, jsonOut)
		}
	},
}

var documentDeleteCmd = &cobra.Command{
	Use:     "delete <document-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a document",
	Long:    `Delete (trash) a document. Reversible with 'document unarchive'.`,
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

		resp, err := api.DocumentDelete(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete document: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.DocumentDelete == nil || !resp.DocumentDelete.Success {
			output.Error("Failed to delete document", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted document %s", args[0]), plaintext, jsonOut)
		}
	},
}

var documentUnarchiveCmd = &cobra.Command{
	Use:     "unarchive <document-id>",
	Aliases: []string{"restore"},
	Short:   "Restore a deleted document",
	Long:    `Restore a previously deleted (trashed) document.`,
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

		resp, err := api.DocumentUnarchive(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to unarchive document: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.DocumentUnarchive == nil || !resp.DocumentUnarchive.Success {
			output.Error("Failed to unarchive document", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Unarchived document %s", args[0]), plaintext, jsonOut)
		}
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
