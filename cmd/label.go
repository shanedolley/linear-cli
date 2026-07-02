package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var labelCmd = &cobra.Command{
	Use:   "label",
	Short: "Manage Linear issue labels",
	Long: `Manage Linear issue labels: the labels applied to issues, scoped to a
team or shared across the whole workspace.

Retiring a label keeps it visible but stops it being applied to new issues;
restore reverses that. Deleting removes the label entirely.

Examples:
  lincli label list                       # List labels
  lincli label list --team ENG            # List a team's labels
  lincli label get LABEL-ID
  lincli label create --name "Bug" --color "#EB5757"
  lincli label create --name "Backend" --team ENG
  lincli label update LABEL-ID --name "Defect"
  lincli label retire LABEL-ID
  lincli label restore LABEL-ID
  lincli label delete LABEL-ID`,
}

var labelListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List issue labels",
	Long:    `List issue labels, optionally filtered by team.`,
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

		filter := &api.IssueLabelFilter{}
		if team, _ := cmd.Flags().GetString("team"); team != "" {
			teamID, err := resolveTeam(ctx, client, cache, team)
			if err != nil {
				return err
			}
			filter.Team = &api.NullableTeamFilter{Id: &api.IDComparator{Eq: &teamID}}
		}

		resp, err := api.ListIssueLabels(ctx, client, filter, limitPtr)
		if err != nil {
			return fmt.Errorf("Failed to list labels: %v", err)
		}

		if len(resp.IssueLabels.Nodes) == 0 {
			output.Info("No labels found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.IssueLabels.Nodes)
			return nil
		}

		if plaintext {
			fmt.Println("# Labels")
			for _, node := range resp.IssueLabels.Nodes {
				f := node.LabelListFields
				fmt.Printf("## %s\n", f.Name)
				fmt.Printf("- **ID**: %s\n", f.Id)
				fmt.Printf("- **Color**: %s\n", f.Color)
				fmt.Printf("- **Scope**: %s\n", labelScope(f))
				if f.Description != nil && *f.Description != "" {
					fmt.Printf("- **Description**: %s\n", *f.Description)
				}
				fmt.Println()
			}
			fmt.Printf("\nTotal: %d labels\n", len(resp.IssueLabels.Nodes))
			return nil
		}

		headers := []string{"ID", "Name", "Color", "Scope", "Parent"}
		rows := make([][]string, len(resp.IssueLabels.Nodes))
		for i, node := range resp.IssueLabels.Nodes {
			f := node.LabelListFields
			parent := ""
			if f.Parent != nil {
				parent = f.Parent.Name
			}
			rows[i] = []string{
				f.Id,
				truncateString(f.Name, 30),
				f.Color,
				labelScope(f),
				parent,
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d labels\n", color.New(color.FgGreen).Sprint("✓"), len(resp.IssueLabels.Nodes))
		}
		return nil
	},
}

var labelGetCmd = &cobra.Command{
	Use:     "get <label-id>",
	Aliases: []string{"show"},
	Short:   "Get label details",
	Long:    `Get detailed information about an issue label.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.GetIssueLabel(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to get label: %v", err)
		}
		if resp.IssueLabel == nil {
			return fmt.Errorf("Label not found: %s", args[0])
		}
		f := resp.IssueLabel.LabelListFields

		if jsonOut {
			output.JSON(resp.IssueLabel)
			return nil
		}

		if plaintext {
			fmt.Printf("# %s\n\n", f.Name)
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Color**: %s\n", f.Color)
			fmt.Printf("- **Scope**: %s\n", labelScope(f))
			if f.Parent != nil {
				fmt.Printf("- **Parent**: %s\n", f.Parent.Name)
			}
			if f.Description != nil && *f.Description != "" {
				fmt.Printf("- **Description**: %s\n", *f.Description)
			}
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Label:"), f.Name)
		fmt.Printf("  ID:    %s\n", f.Id)
		fmt.Printf("  Color: %s\n", f.Color)
		fmt.Printf("  Scope: %s\n", labelScope(f))
		if f.Parent != nil {
			fmt.Printf("  Parent: %s\n", f.Parent.Name)
		}
		if f.Description != nil && *f.Description != "" {
			fmt.Printf("  Description: %s\n", *f.Description)
		}
		return nil
	},
}

var labelCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a new label",
	Long:    `Create a new issue label. Without --team the label is workspace-wide.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return errors.New("Name is required (--name)")
		}

		input := api.IssueLabelCreateInput{Name: name}

		if c, _ := cmd.Flags().GetString("color"); c != "" {
			input.Color = &c
		}
		if description, _ := cmd.Flags().GetString("description"); description != "" {
			input.Description = &description
		}
		if isGroup, _ := cmd.Flags().GetBool("group"); isGroup {
			input.IsGroup = &isGroup
		}
		if team, _ := cmd.Flags().GetString("team"); team != "" {
			teamID, err := resolveTeam(ctx, client, cache, team)
			if err != nil {
				return err
			}
			input.TeamId = &teamID
		}
		if parent, _ := cmd.Flags().GetString("parent"); parent != "" {
			parentID, err := resolveLabel(ctx, client, cache, parent)
			if err != nil {
				return fmt.Errorf("Failed to find parent label '%s': %v", parent, err)
			}
			input.ParentId = &parentID
		}

		resp, err := api.IssueLabelCreate(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to create label: %v", err)
		}
		if !resp.IssueLabelCreate.Success {
			return errors.New("Failed to create label")
		}

		if jsonOut {
			output.JSON(resp.IssueLabelCreate.IssueLabel)
		} else {
			output.Success(fmt.Sprintf("Created label %s", resp.IssueLabelCreate.IssueLabel.LabelListFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var labelUpdateCmd = &cobra.Command{
	Use:   "update <label-id>",
	Short: "Update a label",
	Long:  `Update an issue label's name, color, description, or parent.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		input := api.IssueLabelUpdateInput{}
		hasUpdates := false

		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
			hasUpdates = true
		}
		if cmd.Flags().Changed("color") {
			c, _ := cmd.Flags().GetString("color")
			input.Color = &c
			hasUpdates = true
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
			hasUpdates = true
		}
		if cmd.Flags().Changed("parent") {
			parent, _ := cmd.Flags().GetString("parent")
			parentID, err := resolveLabel(ctx, client, cache, parent)
			if err != nil {
				return fmt.Errorf("Failed to find parent label '%s': %v", parent, err)
			}
			input.ParentId = &parentID
			hasUpdates = true
		}

		if !hasUpdates {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.IssueLabelUpdate(ctx, client, args[0], &input)
		if err != nil {
			return fmt.Errorf("Failed to update label: %v", err)
		}
		if !resp.IssueLabelUpdate.Success {
			return errors.New("Failed to update label")
		}

		if jsonOut {
			output.JSON(resp.IssueLabelUpdate.IssueLabel)
		} else {
			output.Success(fmt.Sprintf("Updated label %s", resp.IssueLabelUpdate.IssueLabel.LabelListFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var labelDeleteCmd = &cobra.Command{
	Use:   "delete <label-id>",
	Short: "Delete a label",
	Long:  `Delete an issue label. This action executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.IssueLabelDelete(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete label: %v", err)
		}
		if !resp.IssueLabelDelete.Success {
			return errors.New("Failed to delete label")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.IssueLabelDelete.EntityId})
		} else {
			output.Success(fmt.Sprintf("Deleted label %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var labelRetireCmd = &cobra.Command{
	Use:   "retire <label-id>",
	Short: "Retire a label",
	Long:  `Retire a label. It stays visible but cannot be applied to new issues. Reversible with 'label restore'.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLabelStateChange(cmd, args[0], "retire")
	},
}

var labelRestoreCmd = &cobra.Command{
	Use:   "restore <label-id>",
	Short: "Restore a retired label",
	Long:  `Restore a previously retired label, making it available for use again.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLabelStateChange(cmd, args[0], "restore")
	},
}

// runLabelStateChange handles the retire/restore commands, which share the
// same shape: resolve nothing, call the mutation, report success.
func runLabelStateChange(cmd *cobra.Command, id, action string) error {
	plaintext := viper.GetBool("plaintext")
	jsonOut := viper.GetBool("json")

	client, err := newGraphQLClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	var success bool
	if action == "retire" {
		resp, err := api.IssueLabelRetire(ctx, client, id)
		if err != nil {
			return fmt.Errorf("Failed to retire label: %v", err)
		}
		success = resp.IssueLabelRetire.Success
	} else {
		resp, err := api.IssueLabelRestore(ctx, client, id)
		if err != nil {
			return fmt.Errorf("Failed to restore label: %v", err)
		}
		success = resp.IssueLabelRestore.Success
	}

	if !success {
		return fmt.Errorf("Failed to %s label", action)
	}

	if jsonOut {
		output.JSON(map[string]interface{}{"success": true, "id": id})
	} else {
		output.Success(fmt.Sprintf("%sd label %s", capitalizeFirst(action), id), plaintext, jsonOut)
	}
	return nil
}

// labelScope reports whether a label is team-scoped or workspace-wide.
func labelScope(f api.LabelListFields) string {
	if f.Team != nil {
		return "team " + f.Team.Key
	}
	return "workspace"
}

// capitalizeFirst upper-cases the first byte of s (ASCII verbs like
// "retire"/"restore"), for building success messages.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func init() {
	rootCmd.AddCommand(labelCmd)
	labelCmd.AddCommand(labelListCmd)
	labelCmd.AddCommand(labelGetCmd)
	labelCmd.AddCommand(labelCreateCmd)
	labelCmd.AddCommand(labelUpdateCmd)
	labelCmd.AddCommand(labelDeleteCmd)
	labelCmd.AddCommand(labelRetireCmd)
	labelCmd.AddCommand(labelRestoreCmd)

	labelListCmd.Flags().StringP("team", "t", "", "Filter by team key")
	labelListCmd.Flags().IntP("limit", "l", 50, "Maximum number of labels to fetch")

	labelCreateCmd.Flags().String("name", "", "Label name (required)")
	labelCreateCmd.Flags().String("color", "", "Label color as a HEX string (e.g. #EB5757)")
	labelCreateCmd.Flags().StringP("description", "d", "", "Label description")
	labelCreateCmd.Flags().StringP("team", "t", "", "Team key (omit for a workspace-wide label)")
	labelCreateCmd.Flags().String("parent", "", "Parent label name or ID")
	labelCreateCmd.Flags().Bool("group", false, "Create the label as a group (container for child labels)")

	labelUpdateCmd.Flags().String("name", "", "Label name")
	labelUpdateCmd.Flags().String("color", "", "Label color as a HEX string (e.g. #EB5757)")
	labelUpdateCmd.Flags().StringP("description", "d", "", "Label description")
	labelUpdateCmd.Flags().String("parent", "", "Parent label name or ID")
}
