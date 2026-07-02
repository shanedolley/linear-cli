package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
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

		resp, err := api.ListCustomViews(ctx, client, nil, limitPtr, nil, nil)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list views: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if resp.CustomViews == nil || len(resp.CustomViews.Nodes) == 0 {
			output.Info("No custom views found", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(resp.CustomViews.Nodes)
			return
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
	},
}

var viewGetCmd = &cobra.Command{
	Use:     "get <view-id>",
	Aliases: []string{"show"},
	Short:   "Get a custom view",
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

		resp, err := api.GetCustomView(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get view: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.CustomView == nil {
			output.Error(fmt.Sprintf("View not found: %s", args[0]), plaintext, jsonOut)
			os.Exit(1)
		}
		f := resp.CustomView.CustomViewFields

		if jsonOut {
			output.JSON(resp.CustomView)
			return
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
			return
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
	},
}

var viewCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a custom view",
	Long:    `Create a custom view. --name is required. Filters are edited in the web app.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			output.Error("Name is required (--name)", plaintext, jsonOut)
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
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.TeamId = &teamID
		}

		resp, err := api.CustomViewCreate(ctx, client, input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create view: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.CustomViewCreate == nil || !resp.CustomViewCreate.Success {
			output.Error("Failed to create view", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.CustomViewCreate.CustomView)
		} else {
			output.Success(fmt.Sprintf("Created view %s", resp.CustomViewCreate.CustomView.CustomViewFields.Name), plaintext, jsonOut)
		}
	},
}

var viewUpdateCmd = &cobra.Command{
	Use:   "update <view-id>",
	Short: "Update a custom view",
	Long:  `Update a view's name, description, icon, color, or sharing.`,
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
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.CustomViewUpdate(context.Background(), client, args[0], input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update view: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.CustomViewUpdate == nil || !resp.CustomViewUpdate.Success {
			output.Error("Failed to update view", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.CustomViewUpdate.CustomView)
		} else {
			output.Success(fmt.Sprintf("Updated view %s", args[0]), plaintext, jsonOut)
		}
	},
}

var viewDeleteCmd = &cobra.Command{
	Use:     "delete <view-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a custom view",
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

		resp, err := api.CustomViewDelete(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete view: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.CustomViewDelete == nil || !resp.CustomViewDelete.Success {
			output.Error("Failed to delete view", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted view %s", args[0]), plaintext, jsonOut)
		}
	},
}

func init() {
	rootCmd.AddCommand(viewCmd)
	viewCmd.AddCommand(viewListCmd)
	viewCmd.AddCommand(viewGetCmd)
	viewCmd.AddCommand(viewCreateCmd)
	viewCmd.AddCommand(viewUpdateCmd)
	viewCmd.AddCommand(viewDeleteCmd)

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
