package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search across Linear",
	Long: `Search across Linear. 'search projects' runs a text search over projects;
'search semantic' runs an AI search across issues, projects, initiatives, and
documents.

Issue and document text search live on their own commands ('issue search',
'document search').

Examples:
  lincli search projects "billing"
  lincli search semantic "flaky checkout tests" --types issue,project`,
}

var searchProjectsCmd = &cobra.Command{
	Use:   "projects <term>",
	Short: "Search projects by text",
	Long:  `Full-text search across projects, ranked by relevance.`,
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
		includeComments, _ := cmd.Flags().GetBool("include-comments")

		resp, err := api.SearchProjects(ctx, client, args[0], limitPtr, &includeComments)
		if err != nil {
			return fmt.Errorf("Failed to search projects: %v", err)
		}

		if len(resp.SearchProjects.Nodes) == 0 {
			output.Info("No matching projects found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.SearchProjects.Nodes)
			return nil
		}

		headers := []string{"Name", "Description", "URL"}
		rows := make([][]string, len(resp.SearchProjects.Nodes))
		for i, node := range resp.SearchProjects.Nodes {
			rows[i] = []string{
				truncateString(node.Name, 35),
				truncateString(node.Description, 45),
				node.Url,
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d of %.0f matches\n", color.New(color.FgGreen).Sprint("✓"), len(resp.SearchProjects.Nodes), resp.SearchProjects.TotalCount)
		}
		return nil
	},
}

var searchSemanticCmd = &cobra.Command{
	Use:   "semantic <query>",
	Short: "AI search across issues, projects, initiatives, and documents",
	Long: `Run Linear's AI (semantic) search across issues, projects, initiatives, and
documents. Restrict the result types with --types.

Examples:
  lincli search semantic "onboarding regressions"
  lincli search semantic "q3 goals" --types project,initiative`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		typesRaw, _ := cmd.Flags().GetString("types")
		types, err := parseSemanticTypes(typesRaw)
		if err != nil {
			return err
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		maxResults, _ := cmd.Flags().GetInt("limit")
		var maxPtr *int
		if maxResults > 0 {
			maxPtr = &maxResults
		}

		resp, err := api.SemanticSearch(ctx, client, args[0], maxPtr, types)
		if err != nil {
			return fmt.Errorf("Failed to run semantic search: %v", err)
		}

		if len(resp.SemanticSearch.Results) == 0 {
			output.Info("No matching results found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.SemanticSearch.Results)
			return nil
		}

		headers := []string{"Type", "Match", "URL"}
		rows := make([][]string, 0, len(resp.SemanticSearch.Results))
		for _, r := range resp.SemanticSearch.Results {
			label, url := "", ""
			switch {
			case r.Issue != nil:
				label = fmt.Sprintf("%s %s", r.Issue.Identifier, r.Issue.Title)
				url = r.Issue.Url
			case r.Project != nil:
				label = r.Project.Name
				url = r.Project.Url
			case r.Initiative != nil:
				label = r.Initiative.Name
				url = r.Initiative.Url
			case r.Document != nil:
				label = r.Document.Title
				url = r.Document.Url
			}
			rows = append(rows, []string{string(r.Type), truncateString(label, 50), url})
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d results\n", color.New(color.FgGreen).Sprint("✓"), len(resp.SemanticSearch.Results))
		}
		return nil
	},
}

// parseSemanticTypes parses the --types flag: a comma-separated subset of
// issue, project, initiative, document, matched case-insensitively. An empty
// string returns nil, meaning "all types" (no server-side filter). An
// unrecognized value returns an error listing the valid types.
func parseSemanticTypes(csv string) ([]api.SemanticSearchResultType, error) {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return nil, nil
	}

	var types []api.SemanticSearchResultType
	for _, part := range strings.Split(csv, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "issue":
			types = append(types, api.SemanticSearchResultTypeIssue)
		case "project":
			types = append(types, api.SemanticSearchResultTypeProject)
		case "initiative":
			types = append(types, api.SemanticSearchResultTypeInitiative)
		case "document":
			types = append(types, api.SemanticSearchResultTypeDocument)
		default:
			return nil, fmt.Errorf("invalid type %q: must be one of issue, project, initiative, document", strings.TrimSpace(part))
		}
	}
	return types, nil
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.AddCommand(searchProjectsCmd)
	searchCmd.AddCommand(searchSemanticCmd)

	searchProjectsCmd.Flags().IntP("limit", "l", 25, "Maximum number of results to fetch")
	searchProjectsCmd.Flags().Bool("include-comments", false, "Also search project comments")

	searchSemanticCmd.Flags().IntP("limit", "l", 25, "Maximum number of results to return")
	searchSemanticCmd.Flags().String("types", "", "Comma-separated result types: issue, project, initiative, document (default: all)")
}
