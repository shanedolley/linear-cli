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

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage external links on initiatives and projects",
	Long: `Manage external links: labelled URLs attached to an initiative or a project,
shown alongside its documents and resources.

The parent (--initiative or --project) is set when the link is created; update
and remove take the link ID from 'add --json' or 'get'.

Examples:
  lincli link add --project "API" --url https://status.example.com --label "Status page"
  lincli link update <id> --label "Status"
  lincli link remove <id>`,
}

var linkAddCmd = &cobra.Command{
	Use:     "add",
	Aliases: []string{"create"},
	Short:   "Add an external link to an initiative or project",
	Long: `Add an external link to exactly one parent: pass --initiative or --project
(by name or ID), plus --url and --label.

Only initiative and project are exposed. The API's other parents (team,
release, cycle) are tagged internal, so they are intentionally left out.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		initiativeRef, _ := cmd.Flags().GetString("initiative")
		projectRef, _ := cmd.Flags().GetString("project")
		parentKind, parentRef, err := validateLinkParent(initiativeRef, projectRef)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		url, _ := cmd.Flags().GetString("url")
		if url == "" {
			output.Error("URL is required (--url)", plaintext, jsonOut)
			os.Exit(1)
		}
		label, _ := cmd.Flags().GetString("label")
		if label == "" {
			output.Error("Label is required (--label)", plaintext, jsonOut)
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

		input := &api.EntityExternalLinkCreateInput{Url: url, Label: label}
		if parentKind == "initiative" {
			id, err := resolveInitiative(ctx, client, cache, parentRef)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.InitiativeId = &id
		} else {
			id, err := resolveProject(ctx, client, cache, parentRef)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.ProjectId = &id
		}

		resp, err := api.EntityExternalLinkCreate(ctx, client, input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create link: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.EntityExternalLinkCreate == nil || !resp.EntityExternalLinkCreate.Success {
			output.Error("Failed to create link", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.EntityExternalLinkCreate.EntityExternalLink)
		} else {
			output.Success(fmt.Sprintf("Added link %s", resp.EntityExternalLinkCreate.EntityExternalLink.EntityExternalLinkFields.Id), plaintext, jsonOut)
		}
	},
}

var linkUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an external link's URL or label",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		input := &api.EntityExternalLinkUpdateInput{}
		changed := false
		if cmd.Flags().Changed("url") {
			url, _ := cmd.Flags().GetString("url")
			input.Url = &url
			changed = true
		}
		if cmd.Flags().Changed("label") {
			label, _ := cmd.Flags().GetString("label")
			input.Label = &label
			changed = true
		}
		if !changed {
			output.Error("No updates specified. Use --url or --label.", plaintext, jsonOut)
			os.Exit(1)
		}

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		resp, err := api.EntityExternalLinkUpdate(context.Background(), client, args[0], input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update link: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.EntityExternalLinkUpdate == nil || !resp.EntityExternalLinkUpdate.Success {
			output.Error("Failed to update link", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.EntityExternalLinkUpdate.EntityExternalLink)
		} else {
			output.Success(fmt.Sprintf("Updated link %s", args[0]), plaintext, jsonOut)
		}
	},
}

var linkRemoveCmd = &cobra.Command{
	Use:     "remove <id>",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove an external link",
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
		resp, err := api.EntityExternalLinkDelete(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to remove link: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.EntityExternalLinkDelete == nil || !resp.EntityExternalLinkDelete.Success {
			output.Error("Failed to remove link", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Removed link %s", args[0]), plaintext, jsonOut)
		}
	},
}

var linkGetCmd = &cobra.Command{
	Use:     "get <id>",
	Aliases: []string{"show"},
	Short:   "Get an external link",
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
		resp, err := api.GetEntityExternalLink(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get link: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		f := resp.EntityExternalLink.EntityExternalLinkFields

		if jsonOut {
			output.JSON(resp.EntityExternalLink)
			return
		}

		parent := ""
		if f.Project != nil {
			parent = fmt.Sprintf("project %s", f.Project.Name)
		} else if f.Initiative != nil {
			parent = fmt.Sprintf("initiative %s", f.Initiative.Name)
		}

		if plaintext {
			fmt.Printf("# %s\n", f.Label)
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **URL**: %s\n", f.Url)
			if parent != "" {
				fmt.Printf("- **Parent**: %s\n", parent)
			}
			return
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Link:"), f.Label)
		fmt.Printf("  ID:     %s\n", f.Id)
		fmt.Printf("  URL:    %s\n", f.Url)
		if parent != "" {
			fmt.Printf("  Parent: %s\n", parent)
		}
	},
}

// validateLinkParent checks that exactly one parent reference is set for an
// external link and returns its kind ("initiative" or "project") and the
// reference. Zero or both references return an error.
func validateLinkParent(initiativeRef, projectRef string) (kind, ref string, err error) {
	switch {
	case initiativeRef != "" && projectRef != "":
		return "", "", fmt.Errorf("pass only one parent: --initiative or --project, not both")
	case initiativeRef != "":
		return "initiative", initiativeRef, nil
	case projectRef != "":
		return "project", projectRef, nil
	default:
		return "", "", fmt.Errorf("a parent is required: pass --initiative or --project")
	}
}

func init() {
	rootCmd.AddCommand(linkCmd)
	linkCmd.AddCommand(linkAddCmd)
	linkCmd.AddCommand(linkUpdateCmd)
	linkCmd.AddCommand(linkRemoveCmd)
	linkCmd.AddCommand(linkGetCmd)

	linkAddCmd.Flags().String("initiative", "", "Parent initiative (name or ID)")
	linkAddCmd.Flags().String("project", "", "Parent project (name or ID)")
	linkAddCmd.Flags().String("url", "", "Link URL (required)")
	linkAddCmd.Flags().String("label", "", "Link label (required)")

	linkUpdateCmd.Flags().String("url", "", "New link URL")
	linkUpdateCmd.Flags().String("label", "", "New link label")
}
