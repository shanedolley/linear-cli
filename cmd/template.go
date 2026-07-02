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

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage issue and project templates",
	Long: `Manage templates: reusable issue and project skeletons.

Examples:
  lincli template list
  lincli template list --template-type issue
  lincli template get TEMPLATE-ID
  lincli template create --name "Bug report" --template-type issue --team ENG --data '{"title":"Bug: "}'
  lincli template update TEMPLATE-ID --name "Bug"
  lincli template delete TEMPLATE-ID

Apply a template when creating an issue or project with the --template flag:
  lincli issue create --title "Login bug" --team ENG --template "Bug report"
  lincli project create --name "Q3 launch" --team ENG --template "Launch"`,
}

var templateListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List templates",
	Long:    `List workspace templates. --template-type filters to issue or project templates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.ListTemplates(ctx, client)
		if err != nil {
			return fmt.Errorf("Failed to list templates: %v", err)
		}

		// The `templates` query returns all templates with no server-side filter,
		// so --template-type narrows the list client-side.
		typeFilter := preferFlag(cmd, "template-type", "type")
		typeFilter = strings.ToLower(strings.TrimSpace(typeFilter))

		nodes := resp.Templates
		filtered := make([]*api.ListTemplatesTemplatesTemplate, 0, len(nodes))
		for _, node := range nodes {
			if typeFilter != "" && !strings.EqualFold(node.TemplateFields.Type, typeFilter) {
				continue
			}
			filtered = append(filtered, node)
		}

		if len(filtered) == 0 {
			output.Info("No templates found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(filtered)
			return nil
		}

		headers := []string{"ID", "Name", "Type", "Team"}
		rows := make([][]string, len(filtered))
		for i, node := range filtered {
			f := node.TemplateFields
			team := "workspace"
			if f.Team != nil {
				team = f.Team.Key
			}
			rows[i] = []string{
				f.Id,
				truncateString(f.Name, 40),
				f.Type,
				team,
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d templates\n", color.New(color.FgGreen).Sprint("✓"), len(filtered))
		}
		return nil
	},
}

var templateGetCmd = &cobra.Command{
	Use:     "get <template-id>",
	Aliases: []string{"show"},
	Short:   "Get a template",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		resp, err := api.GetTemplate(context.Background(), client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to get template: %v", err)
		}
		if resp.Template == nil {
			return fmt.Errorf("Template not found: %s", args[0])
		}
		f := resp.Template.TemplateFields

		if jsonOut {
			output.JSON(resp.Template)
			return nil
		}

		if plaintext {
			fmt.Printf("# %s\n", f.Name)
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Type**: %s\n", f.Type)
			if f.Description != nil && *f.Description != "" {
				fmt.Printf("- **Description**: %s\n", *f.Description)
			}
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Template:"), f.Name)
		fmt.Printf("  ID:   %s\n", f.Id)
		fmt.Printf("  Type: %s\n", f.Type)
		if f.Team != nil {
			fmt.Printf("  Team: %s\n", f.Team.Key)
		}
		if f.Description != nil && *f.Description != "" {
			fmt.Printf("  Description: %s\n", *f.Description)
		}
		return nil
	},
}

var templateCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a template",
	Long: `Create a template. --name, --template-type (issue or project), and --data are required.

--data is the template's JSON payload (pre-filled entity attributes). lincli
passes it through unchanged, so it must be valid JSON.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return errors.New("Name is required (--name)")
		}
		templateType := preferFlag(cmd, "template-type", "type")
		if templateType == "" {
			return errors.New("Type is required (--template-type: issue or project)")
		}
		dataStr, _ := cmd.Flags().GetString("data")
		if dataStr == "" {
			return errors.New("Template data is required (--data, a JSON string)")
		}
		templateData, err := parseTemplateData(dataStr)
		if err != nil {
			return err
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		input := &api.TemplateCreateInput{
			Name:         name,
			Type:         templateType,
			TemplateData: templateData,
		}

		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
		}
		if team, _ := cmd.Flags().GetString("team"); team != "" {
			teamID, err := resolveTeam(ctx, client, cache, team)
			if err != nil {
				return err
			}
			input.TeamId = &teamID
		}

		resp, err := api.TemplateCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to create template: %v", err)
		}
		if resp.TemplateCreate == nil || !resp.TemplateCreate.Success {
			return errors.New("Failed to create template")
		}

		if jsonOut {
			output.JSON(resp.TemplateCreate.Template)
		} else {
			output.Success(fmt.Sprintf("Created template %s", resp.TemplateCreate.Template.TemplateFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var templateUpdateCmd = &cobra.Command{
	Use:   "update <template-id>",
	Short: "Update a template",
	Long:  `Update a template's name, description, or JSON data.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		input := &api.TemplateUpdateInput{}
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
		if cmd.Flags().Changed("data") {
			dataStr, _ := cmd.Flags().GetString("data")
			templateData, err := parseTemplateData(dataStr)
			if err != nil {
				return err
			}
			input.TemplateData = &templateData
			changed = true
		}

		if !changed {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.TemplateUpdate(context.Background(), client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to update template: %v", err)
		}
		if resp.TemplateUpdate == nil || !resp.TemplateUpdate.Success {
			return errors.New("Failed to update template")
		}

		if jsonOut {
			output.JSON(resp.TemplateUpdate.Template)
		} else {
			output.Success(fmt.Sprintf("Updated template %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var templateDeleteCmd = &cobra.Command{
	Use:     "delete <template-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a template",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		resp, err := api.TemplateDelete(context.Background(), client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete template: %v", err)
		}
		if resp.TemplateDelete == nil || !resp.TemplateDelete.Success {
			return errors.New("Failed to delete template")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted template %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// parseTemplateData unmarshals a --data JSON string into the interface{} value
// the template mutations expect, wrapping a parse failure in a caller-friendly
// error. The RunE closures handle output/error return, keeping this helper
// side-effect free like the other helpers in this package.
func parseTemplateData(dataStr string) (interface{}, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return nil, fmt.Errorf("invalid --data JSON: %v", err)
	}
	return data, nil
}

func init() {
	rootCmd.AddCommand(templateCmd)
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateGetCmd)
	templateCmd.AddCommand(templateCreateCmd)
	templateCmd.AddCommand(templateUpdateCmd)
	templateCmd.AddCommand(templateDeleteCmd)

	templateListCmd.Flags().String("template-type", "", "Filter by template type: issue or project")
	templateListCmd.Flags().String("type", "", "Deprecated alias for --template-type")
	_ = templateListCmd.Flags().MarkHidden("type")

	templateCreateCmd.Flags().String("name", "", "Template name (required)")
	templateCreateCmd.Flags().String("template-type", "", "Template type: issue or project (required)")
	templateCreateCmd.Flags().String("type", "", "Deprecated alias for --template-type")
	_ = templateCreateCmd.Flags().MarkHidden("type")
	templateCreateCmd.Flags().String("data", "", "Template payload as a JSON string (required)")
	templateCreateCmd.Flags().StringP("description", "d", "", "Template description")
	templateCreateCmd.Flags().StringP("team", "t", "", "Scope the template to a team (key, name, or ID)")

	templateUpdateCmd.Flags().String("name", "", "New template name")
	templateUpdateCmd.Flags().StringP("description", "d", "", "New template description")
	templateUpdateCmd.Flags().String("data", "", "New template payload as a JSON string")
}
