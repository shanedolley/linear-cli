package cmd

import (
	"context"
	"encoding/json"
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

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage issue and project templates",
	Long: `Manage templates: reusable issue and project skeletons.

Examples:
  lincli template list
  lincli template list --type issue
  lincli template get TEMPLATE-ID
  lincli template create --name "Bug report" --type issue --team ENG --data '{"title":"Bug: "}'
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
	Long:    `List workspace templates. --type filters to issue or project templates.`,
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

		resp, err := api.ListTemplates(ctx, client)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list templates: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		// The `templates` query returns all templates with no server-side filter,
		// so --type narrows the list client-side.
		typeFilter, _ := cmd.Flags().GetString("type")
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
			return
		}

		if jsonOut {
			output.JSON(filtered)
			return
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
	},
}

var templateGetCmd = &cobra.Command{
	Use:     "get <template-id>",
	Aliases: []string{"show"},
	Short:   "Get a template",
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

		resp, err := api.GetTemplate(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get template: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.Template == nil {
			output.Error(fmt.Sprintf("Template not found: %s", args[0]), plaintext, jsonOut)
			os.Exit(1)
		}
		f := resp.Template.TemplateFields

		if jsonOut {
			output.JSON(resp.Template)
			return
		}

		if plaintext {
			fmt.Printf("# %s\n", f.Name)
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Type**: %s\n", f.Type)
			if f.Description != nil && *f.Description != "" {
				fmt.Printf("- **Description**: %s\n", *f.Description)
			}
			return
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
	},
}

var templateCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a template",
	Long: `Create a template. --name, --type (issue or project), and --data are required.

--data is the template's JSON payload (pre-filled entity attributes). lincli
passes it through unchanged, so it must be valid JSON.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			output.Error("Name is required (--name)", plaintext, jsonOut)
			os.Exit(1)
		}
		templateType, _ := cmd.Flags().GetString("type")
		if templateType == "" {
			output.Error("Type is required (--type: issue or project)", plaintext, jsonOut)
			os.Exit(1)
		}
		dataStr, _ := cmd.Flags().GetString("data")
		if dataStr == "" {
			output.Error("Template data is required (--data, a JSON string)", plaintext, jsonOut)
			os.Exit(1)
		}
		templateData, err := parseTemplateData(dataStr)
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
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.TeamId = &teamID
		}

		resp, err := api.TemplateCreate(ctx, client, input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create template: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.TemplateCreate == nil || !resp.TemplateCreate.Success {
			output.Error("Failed to create template", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.TemplateCreate.Template)
		} else {
			output.Success(fmt.Sprintf("Created template %s", resp.TemplateCreate.Template.TemplateFields.Name), plaintext, jsonOut)
		}
	},
}

var templateUpdateCmd = &cobra.Command{
	Use:   "update <template-id>",
	Short: "Update a template",
	Long:  `Update a template's name, description, or JSON data.`,
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
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.TemplateData = &templateData
			changed = true
		}

		if !changed {
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.TemplateUpdate(context.Background(), client, args[0], input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update template: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.TemplateUpdate == nil || !resp.TemplateUpdate.Success {
			output.Error("Failed to update template", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.TemplateUpdate.Template)
		} else {
			output.Success(fmt.Sprintf("Updated template %s", args[0]), plaintext, jsonOut)
		}
	},
}

var templateDeleteCmd = &cobra.Command{
	Use:     "delete <template-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a template",
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

		resp, err := api.TemplateDelete(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete template: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.TemplateDelete == nil || !resp.TemplateDelete.Success {
			output.Error("Failed to delete template", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted template %s", args[0]), plaintext, jsonOut)
		}
	},
}

// parseTemplateData unmarshals a --data JSON string into the interface{} value
// the template mutations expect, wrapping a parse failure in a caller-friendly
// error. The Run closures handle output/exit, keeping this helper side-effect
// free like the other helpers in this package.
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

	templateListCmd.Flags().String("type", "", "Filter by template type: issue or project")

	templateCreateCmd.Flags().String("name", "", "Template name (required)")
	templateCreateCmd.Flags().String("type", "", "Template type: issue or project (required)")
	templateCreateCmd.Flags().String("data", "", "Template payload as a JSON string (required)")
	templateCreateCmd.Flags().StringP("description", "d", "", "Template description")
	templateCreateCmd.Flags().StringP("team", "t", "", "Scope the template to a team (key, name, or ID)")

	templateUpdateCmd.Flags().String("name", "", "New template name")
	templateUpdateCmd.Flags().StringP("description", "d", "", "New template description")
	templateUpdateCmd.Flags().String("data", "", "New template payload as a JSON string")
}
