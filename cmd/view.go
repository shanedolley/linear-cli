package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "Manage custom views",
	Long: `Manage custom views: saved, filtered lists of issues, projects, or initiatives.

lincli covers the view lifecycle (list/get/create/update/delete) and its
metadata (name, description, icon, color, sharing). Edit a view's filters in
the Linear web app.

Examples:
  lincli view list
  lincli view get VIEW-ID
  lincli view create --name "My bugs" --shared
  lincli view update VIEW-ID --description "Open bugs assigned to me"
  lincli view delete VIEW-ID`,
}

var viewListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List custom views",
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

		resp, err := api.ListCustomViews(ctx, client, nil, limitPtr, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list views: %w", err)
		}

		if resp.CustomViews == nil || len(resp.CustomViews.Nodes) == 0 {
			output.Info("No custom views found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.CustomViews.Nodes)
			return nil
		}

		headers := []string{"ID", "Name", "Model", "Shared", "Owner"}
		rows := make([][]string, len(resp.CustomViews.Nodes))
		for i, node := range resp.CustomViews.Nodes {
			f := node.CustomViewFields
			owner := ""
			if f.Owner != nil {
				owner = f.Owner.Name
			}
			rows[i] = []string{
				f.Id,
				truncateString(f.Name, 35),
				f.ModelName,
				fmt.Sprintf("%t", f.Shared),
				owner,
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d views\n", color.New(color.FgGreen).Sprint("✓"), len(resp.CustomViews.Nodes))
		}
		return nil
	},
}

var viewGetCmd = &cobra.Command{
	Use:     "get <view-id>",
	Aliases: []string{"show"},
	Short:   "Get a custom view",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		resp, err := api.GetCustomView(context.Background(), client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to get view: %w", err)
		}
		if resp.CustomView == nil {
			return fmt.Errorf("View not found: %s", args[0])
		}
		f := resp.CustomView.CustomViewFields

		if jsonOut {
			output.JSON(resp.CustomView)
			return nil
		}

		description := ""
		if f.Description != nil {
			description = *f.Description
		}

		if plaintext {
			fmt.Printf("# %s\n", f.Name)
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Model**: %s\n", f.ModelName)
			fmt.Printf("- **Shared**: %t\n", f.Shared)
			if description != "" {
				fmt.Printf("- **Description**: %s\n", description)
			}
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("View:"), f.Name)
		fmt.Printf("  ID:      %s\n", f.Id)
		fmt.Printf("  Model:   %s\n", f.ModelName)
		fmt.Printf("  Shared:  %t\n", f.Shared)
		if f.Owner != nil {
			fmt.Printf("  Owner:   %s\n", f.Owner.Name)
		}
		if description != "" {
			fmt.Printf("  Description: %s\n", description)
		}
		return nil
	},
}

var viewCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a custom view",
	Long:    `Create a custom view. --name is required. Filters are edited in the web app.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return errors.New("Name is required (--name)")
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		input := &api.CustomViewCreateInput{Name: name}

		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
		}
		if cmd.Flags().Changed("icon") {
			icon, _ := cmd.Flags().GetString("icon")
			input.Icon = &icon
		}
		if cmd.Flags().Changed("color") {
			c, _ := cmd.Flags().GetString("color")
			input.Color = &c
		}
		if cmd.Flags().Changed("shared") {
			shared, _ := cmd.Flags().GetBool("shared")
			input.Shared = &shared
		}
		if team, _ := cmd.Flags().GetString("team"); team != "" {
			teamID, err := resolveTeam(ctx, client, cache, team)
			if err != nil {
				return err
			}
			input.TeamId = &teamID
		}

		resp, err := api.CustomViewCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to create view: %w", err)
		}
		if resp.CustomViewCreate == nil || !resp.CustomViewCreate.Success {
			return errors.New("Failed to create view")
		}

		if jsonOut {
			output.JSON(resp.CustomViewCreate.CustomView)
		} else {
			output.Success(fmt.Sprintf("Created view %s", resp.CustomViewCreate.CustomView.CustomViewFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var viewUpdateCmd = &cobra.Command{
	Use:   "update <view-id>",
	Short: "Update a custom view",
	Long:  `Update a view's name, description, icon, color, or sharing.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		input := &api.CustomViewUpdateInput{}
		changed := false

		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
			changed = true
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
			changed = true
		}
		if cmd.Flags().Changed("icon") {
			icon, _ := cmd.Flags().GetString("icon")
			input.Icon = &icon
			changed = true
		}
		if cmd.Flags().Changed("color") {
			c, _ := cmd.Flags().GetString("color")
			input.Color = &c
			changed = true
		}
		if cmd.Flags().Changed("shared") {
			shared, _ := cmd.Flags().GetBool("shared")
			input.Shared = &shared
			changed = true
		}

		if !changed {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.CustomViewUpdate(context.Background(), client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to update view: %w", err)
		}
		if resp.CustomViewUpdate == nil || !resp.CustomViewUpdate.Success {
			return errors.New("Failed to update view")
		}

		if jsonOut {
			output.JSON(resp.CustomViewUpdate.CustomView)
		} else {
			output.Success(fmt.Sprintf("Updated view %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var viewDeleteCmd = &cobra.Command{
	Use:     "delete <view-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a custom view",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		resp, err := api.CustomViewDelete(context.Background(), client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete view: %w", err)
		}
		if resp.CustomViewDelete == nil || !resp.CustomViewDelete.Success {
			return errors.New("Failed to delete view")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted view %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// --- view prefs ---

var viewPrefsCmd = &cobra.Command{
	Use:     "prefs",
	Aliases: []string{"preferences"},
	Short:   "Manage view preferences",
	Long: `Manage view preferences: per-user or per-workspace display settings (layout,
grouping, sorting) for a view, attached to a parent such as a team, project,
custom view, or label.

Preferences are a raw JSON object; edit them in the Linear web app to discover
the shape, then round-trip that JSON here.

Examples:
  lincli view prefs create --scope user --view-type activeIssues --team ENG --preferences '{"grouping":"assignee"}'
  lincli view prefs update <id> --preferences '{"grouping":"priority"}'
  lincli view prefs delete <id>`,
}

var viewPrefsCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a view preferences object",
	Long: `Create a view preferences object. --scope (user or organization), --view-type,
and --preferences (a JSON object) are required. Pass at least one parent
association: --team, --project, --custom-view, or --label.

--preferences must be a non-empty JSON object; the CLI cannot send an empty
{} (the client drops empty objects before the request).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		scopeRaw, _ := cmd.Flags().GetString("scope")
		scope, err := parseViewPreferencesType(scopeRaw)
		if err != nil {
			return err
		}

		viewType, _ := cmd.Flags().GetString("view-type")
		if viewType == "" {
			return errors.New("View type is required (--view-type)")
		}

		prefsRaw, _ := cmd.Flags().GetString("preferences")
		prefs, err := parseJSONObject(prefsRaw)
		if err != nil {
			return fmt.Errorf("Invalid --preferences JSON: %w", err)
		}
		if len(prefs) == 0 {
			return errors.New(`--preferences must be a non-empty JSON object, e.g. '{"grouping":"assignee"}'`)
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		input := &api.ViewPreferencesCreateInput{
			Type:        scope,
			ViewType:    api.ViewType(viewType),
			Preferences: prefs,
		}

		if team, _ := cmd.Flags().GetString("team"); team != "" {
			id, err := resolveTeam(ctx, client, cache, team)
			if err != nil {
				return err
			}
			input.TeamId = &id
		}
		if project, _ := cmd.Flags().GetString("project"); project != "" {
			id, err := resolveProject(ctx, client, cache, project)
			if err != nil {
				return err
			}
			input.ProjectId = &id
		}
		if customView, _ := cmd.Flags().GetString("custom-view"); customView != "" {
			input.CustomViewId = &customView
		}
		if label, _ := cmd.Flags().GetString("label"); label != "" {
			id, err := resolveLabel(ctx, client, cache, label)
			if err != nil {
				return err
			}
			input.LabelId = &id
		}

		if input.TeamId == nil && input.ProjectId == nil && input.CustomViewId == nil && input.LabelId == nil {
			return errors.New("A parent is required: pass --team, --project, --custom-view, or --label")
		}

		resp, err := api.ViewPreferencesCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to create view preferences: %w", err)
		}
		if resp.ViewPreferencesCreate == nil || !resp.ViewPreferencesCreate.Success {
			return errors.New("Failed to create view preferences")
		}

		if jsonOut {
			output.JSON(resp.ViewPreferencesCreate.ViewPreferences)
		} else {
			output.Success(fmt.Sprintf("Created view preferences %s", resp.ViewPreferencesCreate.ViewPreferences.Id), plaintext, jsonOut)
		}
		return nil
	},
}

var viewPrefsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a view preferences object",
	Long:  `Update a view preferences object's --preferences JSON.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		if !cmd.Flags().Changed("preferences") {
			return errors.New("No updates specified. Use --preferences.")
		}
		prefsRaw, _ := cmd.Flags().GetString("preferences")
		prefs, err := parseJSONObject(prefsRaw)
		if err != nil {
			return fmt.Errorf("Invalid --preferences JSON: %w", err)
		}
		if len(prefs) == 0 {
			return errors.New(`--preferences must be a non-empty JSON object, e.g. '{"grouping":"assignee"}'`)
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		input := &api.ViewPreferencesUpdateInput{Preferences: &prefs}

		resp, err := api.ViewPreferencesUpdate(context.Background(), client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to update view preferences: %w", err)
		}
		if resp.ViewPreferencesUpdate == nil || !resp.ViewPreferencesUpdate.Success {
			return errors.New("Failed to update view preferences")
		}

		if jsonOut {
			output.JSON(resp.ViewPreferencesUpdate.ViewPreferences)
		} else {
			output.Success(fmt.Sprintf("Updated view preferences %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var viewPrefsDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a view preferences object",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		resp, err := api.ViewPreferencesDelete(context.Background(), client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete view preferences: %w", err)
		}
		if resp.ViewPreferencesDelete == nil || !resp.ViewPreferencesDelete.Success {
			return errors.New("Failed to delete view preferences")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted view preferences %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// parseViewPreferencesType parses the --scope flag into a ViewPreferencesType:
// user or organization, matched case-insensitively. An unrecognized value
// returns an error.
func parseViewPreferencesType(s string) (api.ViewPreferencesType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "user":
		return api.ViewPreferencesTypeUser, nil
	case "organization":
		return api.ViewPreferencesTypeOrganization, nil
	default:
		return "", fmt.Errorf("invalid scope %q: must be user or organization", s)
	}
}

// parseJSONObject parses a JSON object string into a map. An empty string
// yields an empty map, which the view-prefs callers reject: the client's
// stripNulls drops empty objects, so an empty preferences value cannot be sent.
func parseJSONObject(s string) (map[string]interface{}, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return map[string]interface{}{}, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func init() {
	rootCmd.AddCommand(viewCmd)
	viewCmd.AddCommand(viewListCmd)
	viewCmd.AddCommand(viewGetCmd)
	viewCmd.AddCommand(viewCreateCmd)
	viewCmd.AddCommand(viewUpdateCmd)
	viewCmd.AddCommand(viewDeleteCmd)

	viewCmd.AddCommand(viewPrefsCmd)
	viewPrefsCmd.AddCommand(viewPrefsCreateCmd)
	viewPrefsCmd.AddCommand(viewPrefsUpdateCmd)
	viewPrefsCmd.AddCommand(viewPrefsDeleteCmd)

	viewPrefsCreateCmd.Flags().String("scope", "", "Preference scope: user or organization (required)")
	viewPrefsCreateCmd.Flags().String("view-type", "", "View type the preferences apply to, e.g. activeIssues, board (required)")
	viewPrefsCreateCmd.Flags().String("preferences", "", "Preferences as a JSON object (required)")
	viewPrefsCreateCmd.Flags().StringP("team", "t", "", "Parent team (key or ID)")
	viewPrefsCreateCmd.Flags().String("project", "", "Parent project (name or ID)")
	viewPrefsCreateCmd.Flags().String("custom-view", "", "Parent custom view ID")
	viewPrefsCreateCmd.Flags().String("label", "", "Parent label (name or ID)")

	viewPrefsUpdateCmd.Flags().String("preferences", "", "New preferences as a JSON object")

	viewListCmd.Flags().IntP("limit", "l", 50, "Maximum number of views to fetch")

	viewCreateCmd.Flags().String("name", "", "View name (required)")
	viewCreateCmd.Flags().StringP("description", "d", "", "View description")
	viewCreateCmd.Flags().String("icon", "", "View icon (emoji)")
	viewCreateCmd.Flags().String("color", "", "View color as a HEX string (e.g. #EB5757)")
	viewCreateCmd.Flags().Bool("shared", false, "Share the view with the workspace")
	viewCreateCmd.Flags().StringP("team", "t", "", "Scope the view to a team (key, name, or ID)")

	viewUpdateCmd.Flags().String("name", "", "New view name")
	viewUpdateCmd.Flags().StringP("description", "d", "", "New view description")
	viewUpdateCmd.Flags().String("icon", "", "New view icon (emoji)")
	viewUpdateCmd.Flags().String("color", "", "New view color as a HEX string")
	viewUpdateCmd.Flags().Bool("shared", false, "Whether the view is shared with the workspace")
}
