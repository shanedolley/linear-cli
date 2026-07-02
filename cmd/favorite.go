package cmd

import (
	"context"
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

var favoriteCmd = &cobra.Command{
	Use:     "favorite",
	Aliases: []string{"fav"},
	Short:   "Manage your favorites",
	Long: `Manage your personal favorites: sidebar bookmarks for issues, projects,
cycles, documents, custom views, labels, users, and initiatives.

Examples:
  lincli favorite list
  lincli favorite add --entity issue ENG-123
  lincli favorite add --entity project "Q1 Roadmap"
  lincli favorite remove FAVORITE-ID`,
}

var favoriteListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List your favorites",
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

		resp, err := api.ListFavorites(ctx, client, limitPtr, nil, nil)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list favorites: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if resp.Favorites == nil || len(resp.Favorites.Nodes) == 0 {
			output.Info("No favorites found", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(resp.Favorites.Nodes)
			return
		}

		headers := []string{"ID", "Type", "Title", "Reference"}
		rows := make([][]string, len(resp.Favorites.Nodes))
		for i, node := range resp.Favorites.Nodes {
			f := node.FavoriteFields
			rows[i] = []string{
				f.Id,
				f.Type,
				truncateString(favoriteTitle(f), 40),
				truncateString(favoriteReference(f), 30),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d favorites\n", color.New(color.FgGreen).Sprint("✓"), len(resp.Favorites.Nodes))
		}
	},
}

var favoriteAddCmd = &cobra.Command{
	Use:   "add <reference>",
	Short: "Add a favorite",
	Long: `Add a favorite for an entity selected by --entity.

--entity picks the kind of entity to favorite: issue, project, cycle, document,
view, label, user, or initiative. The reference is resolved for kinds that
support name/email lookup (issue identifier, project, label, user, initiative);
for cycle, document, and view, pass the entity's ID.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		entity, _ := cmd.Flags().GetString("entity")
		if entity == "" {
			output.Error("Entity kind is required (--entity)", plaintext, jsonOut)
			os.Exit(1)
		}
		kind, err := favoriteEntityKind(entity)
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

		ref := args[0]
		input := &api.FavoriteCreateInput{}

		switch kind {
		case "issue":
			// Linear's issueId accepts a UUID or an issue identifier directly.
			input.IssueId = &ref
		case "project":
			id, err := resolveProject(ctx, client, cache, ref)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.ProjectId = &id
		case "label":
			id, err := resolveLabel(ctx, client, cache, ref)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.LabelId = &id
		case "user":
			id, err := resolveUser(ctx, client, cache, ref)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.UserId = &id
		case "initiative":
			id, err := resolveInitiative(ctx, client, cache, ref)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.InitiativeId = &id
		case "cycle":
			input.CycleId = &ref
		case "document":
			input.DocumentId = &ref
		case "view":
			input.CustomViewId = &ref
		}

		resp, err := api.FavoriteCreate(ctx, client, input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to add favorite: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.FavoriteCreate == nil || !resp.FavoriteCreate.Success {
			output.Error("Failed to add favorite", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.FavoriteCreate.Favorite)
		} else {
			output.Success(fmt.Sprintf("Added favorite %s (%s)", resp.FavoriteCreate.Favorite.FavoriteFields.Id, kind), plaintext, jsonOut)
		}
	},
}

var favoriteRemoveCmd = &cobra.Command{
	Use:     "remove <favorite-id>",
	Aliases: []string{"rm"},
	Short:   "Remove a favorite",
	Long:    `Remove a favorite by its ID (from 'favorite list').`,
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

		resp, err := api.FavoriteDelete(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to remove favorite: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.FavoriteDelete == nil || !resp.FavoriteDelete.Success {
			output.Error("Failed to remove favorite", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Removed favorite %s", args[0]), plaintext, jsonOut)
		}
	},
}

// favoriteEntityKind normalizes and validates a --entity value, returning the
// canonical kind. It maps the CLI's kind names to the FavoriteCreateInput field
// each one sets; unknown kinds are rejected with a message listing the valid
// values. "view" is the CLI name for a custom view.
func favoriteEntityKind(kind string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(kind))
	switch k {
	case "issue", "project", "cycle", "document", "view", "label", "user", "initiative":
		return k, nil
	default:
		return "", fmt.Errorf("unknown entity kind %q; valid kinds: issue, project, cycle, document, view, label, user, initiative", kind)
	}
}

// favoriteTitle renders a favorite's title, falling back to its folder name for
// folder favorites, then "(untitled)".
func favoriteTitle(f api.FavoriteFields) string {
	if f.Title != "" {
		return f.Title
	}
	if f.FolderName != nil && *f.FolderName != "" {
		return *f.FolderName
	}
	return "(untitled)"
}

// favoriteReference renders a short human-readable reference to the favorited
// entity (e.g. an issue identifier or project name), or "-" when none of the
// selected entity fields is populated.
func favoriteReference(f api.FavoriteFields) string {
	switch {
	case f.Issue != nil:
		return f.Issue.Identifier
	case f.Project != nil:
		return f.Project.Name
	case f.Cycle != nil:
		if f.Cycle.Name != nil && *f.Cycle.Name != "" {
			return *f.Cycle.Name
		}
		return fmt.Sprintf("Cycle %.0f", f.Cycle.Number)
	case f.Document != nil:
		return f.Document.Title
	case f.CustomView != nil:
		return f.CustomView.Name
	case f.Label != nil:
		return f.Label.Name
	case f.User != nil:
		return f.User.Name
	case f.Initiative != nil:
		return f.Initiative.Name
	default:
		return "-"
	}
}

func init() {
	rootCmd.AddCommand(favoriteCmd)
	favoriteCmd.AddCommand(favoriteListCmd)
	favoriteCmd.AddCommand(favoriteAddCmd)
	favoriteCmd.AddCommand(favoriteRemoveCmd)

	favoriteListCmd.Flags().IntP("limit", "l", 50, "Maximum number of favorites to fetch")

	favoriteAddCmd.Flags().StringP("entity", "e", "", "Entity kind to favorite: issue, project, cycle, document, view, label, user, initiative (required)")
}
