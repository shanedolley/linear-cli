package cmd

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Manage Linear webhooks",
	Long: `Manage Linear webhooks: HTTP endpoints Linear calls when workspace events occur.

Linear fetches the target URL server-side, so 'webhook create' rejects URLs
pointing at internal addresses (localhost, loopback, link-local, or private IP
ranges).

Examples:
  lincli webhook list
  lincli webhook get WEBHOOK-ID
  lincli webhook create --url https://hooks.example.com/linear --resource-types Issue,Comment
  lincli webhook update WEBHOOK-ID --enabled=false
  lincli webhook delete WEBHOOK-ID
  lincli webhook rotate-secret WEBHOOK-ID`,
}

var webhookListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List webhooks",
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

		resp, err := api.ListWebhooks(ctx, client, limitPtr, nil, nil)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list webhooks: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if resp.Webhooks == nil || len(resp.Webhooks.Nodes) == 0 {
			output.Info("No webhooks found", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(resp.Webhooks.Nodes)
			return
		}

		headers := []string{"ID", "Label", "URL", "Enabled", "Resources"}
		rows := make([][]string, len(resp.Webhooks.Nodes))
		for i, node := range resp.Webhooks.Nodes {
			f := node.WebhookFields
			rows[i] = []string{
				f.Id,
				truncateString(webhookLabel(f), 25),
				truncateString(webhookURL(f), 40),
				fmt.Sprintf("%t", f.Enabled),
				truncateString(strings.Join(f.ResourceTypes, ","), 30),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d webhooks\n", color.New(color.FgGreen).Sprint("✓"), len(resp.Webhooks.Nodes))
		}
	},
}

var webhookGetCmd = &cobra.Command{
	Use:     "get <webhook-id>",
	Aliases: []string{"show"},
	Short:   "Get a webhook",
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

		resp, err := api.GetWebhook(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get webhook: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.Webhook == nil {
			output.Error(fmt.Sprintf("Webhook not found: %s", args[0]), plaintext, jsonOut)
			os.Exit(1)
		}
		f := resp.Webhook.WebhookFields

		if jsonOut {
			output.JSON(resp.Webhook)
			return
		}

		if plaintext {
			fmt.Printf("# %s\n", webhookLabel(f))
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **URL**: %s\n", webhookURL(f))
			fmt.Printf("- **Enabled**: %t\n", f.Enabled)
			fmt.Printf("- **Resources**: %s\n", strings.Join(f.ResourceTypes, ", "))
			return
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Webhook:"), webhookLabel(f))
		fmt.Printf("  ID:        %s\n", f.Id)
		fmt.Printf("  URL:       %s\n", webhookURL(f))
		fmt.Printf("  Enabled:   %t\n", f.Enabled)
		fmt.Printf("  Resources: %s\n", strings.Join(f.ResourceTypes, ", "))
		if f.Team != nil {
			fmt.Printf("  Team:      %s\n", f.Team.Key)
		} else if f.AllPublicTeams {
			fmt.Printf("  Team:      all public teams\n")
		}
	},
}

var webhookCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a webhook",
	Long: `Create a webhook. --url and --resource-types are required.

The URL is validated locally before the API call: localhost, loopback,
link-local, and private/internal IP addresses are rejected because Linear
fetches the URL server-side.`,
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

		targetURL, _ := cmd.Flags().GetString("url")
		if targetURL == "" {
			output.Error("URL is required (--url)", plaintext, jsonOut)
			os.Exit(1)
		}
		if err := validateWebhookURL(targetURL); err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resourceTypes, _ := cmd.Flags().GetStringSlice("resource-types")
		if len(resourceTypes) == 0 {
			output.Error("At least one resource type is required (--resource-types)", plaintext, jsonOut)
			os.Exit(1)
		}

		input := &api.WebhookCreateInput{
			Url:           targetURL,
			ResourceTypes: resourceTypes,
		}

		if cmd.Flags().Changed("label") {
			label, _ := cmd.Flags().GetString("label")
			input.Label = &label
		}
		if cmd.Flags().Changed("secret") {
			secret, _ := cmd.Flags().GetString("secret")
			input.Secret = &secret
		}
		if cmd.Flags().Changed("enabled") {
			enabled, _ := cmd.Flags().GetBool("enabled")
			input.Enabled = &enabled
		}
		if cmd.Flags().Changed("all-public-teams") {
			all, _ := cmd.Flags().GetBool("all-public-teams")
			input.AllPublicTeams = &all
		}
		if team, _ := cmd.Flags().GetString("team"); team != "" {
			teamID, err := resolveTeam(ctx, client, cache, team)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.TeamId = &teamID
		}

		resp, err := api.WebhookCreate(ctx, client, input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create webhook: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.WebhookCreate == nil || !resp.WebhookCreate.Success {
			output.Error("Failed to create webhook", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.WebhookCreate.Webhook)
		} else {
			output.Success(fmt.Sprintf("Created webhook %s", resp.WebhookCreate.Webhook.WebhookFields.Id), plaintext, jsonOut)
		}
	},
}

var webhookUpdateCmd = &cobra.Command{
	Use:   "update <webhook-id>",
	Short: "Update a webhook",
	Long:  `Update a webhook's URL, label, resource types, or enabled state.`,
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

		input := &api.WebhookUpdateInput{}
		changed := false

		if cmd.Flags().Changed("url") {
			targetURL, _ := cmd.Flags().GetString("url")
			if err := validateWebhookURL(targetURL); err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.Url = &targetURL
			changed = true
		}
		if cmd.Flags().Changed("label") {
			label, _ := cmd.Flags().GetString("label")
			input.Label = &label
			changed = true
		}
		if cmd.Flags().Changed("resource-types") {
			resourceTypes, _ := cmd.Flags().GetStringSlice("resource-types")
			input.ResourceTypes = resourceTypes
			changed = true
		}
		if cmd.Flags().Changed("enabled") {
			enabled, _ := cmd.Flags().GetBool("enabled")
			input.Enabled = &enabled
			changed = true
		}
		if cmd.Flags().Changed("secret") {
			secret, _ := cmd.Flags().GetString("secret")
			input.Secret = &secret
			changed = true
		}

		if !changed {
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.WebhookUpdate(context.Background(), client, args[0], input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update webhook: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.WebhookUpdate == nil || !resp.WebhookUpdate.Success {
			output.Error("Failed to update webhook", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.WebhookUpdate.Webhook)
		} else {
			output.Success(fmt.Sprintf("Updated webhook %s", args[0]), plaintext, jsonOut)
		}
	},
}

var webhookDeleteCmd = &cobra.Command{
	Use:     "delete <webhook-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a webhook",
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

		resp, err := api.WebhookDelete(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete webhook: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.WebhookDelete == nil || !resp.WebhookDelete.Success {
			output.Error("Failed to delete webhook", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted webhook %s", args[0]), plaintext, jsonOut)
		}
	},
}

var webhookRotateSecretCmd = &cobra.Command{
	Use:   "rotate-secret <webhook-id>",
	Short: "Rotate a webhook's signing secret",
	Long: `Rotate a webhook's signing secret and print the new value once.

The new secret is printed a single time; capture it now. lincli does not store
it, and rotating again replaces it.`,
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

		resp, err := api.WebhookRotateSecret(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to rotate webhook secret: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.WebhookRotateSecret == nil || !resp.WebhookRotateSecret.Success {
			output.Error("Failed to rotate webhook secret", plaintext, jsonOut)
			os.Exit(1)
		}

		secret := resp.WebhookRotateSecret.Secret
		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0], "secret": secret})
			return
		}
		if plaintext {
			fmt.Println(secret)
			return
		}
		fmt.Printf("%s Rotated secret for webhook %s\n", color.New(color.FgGreen).Sprint("✓"), args[0])
		fmt.Printf("  New secret (shown once): %s\n", color.New(color.FgYellow).Sprint(secret))
	},
}

// validateWebhookURL rejects webhook target URLs that point at an internal
// address. Linear fetches the URL server-side, so a localhost, loopback,
// link-local, or private/internal IP target would let a caller probe Linear's
// own network (server-side request forgery). Only http/https URLs with a host
// are accepted; IP-literal hosts are checked against the reserved ranges.
func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL %q: %v", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook URL %q must use http or https", rawURL)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook URL %q has no host", rawURL)
	}

	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return fmt.Errorf("webhook URL host %q is not allowed (internal address)", host)
	}

	// An IP-literal host is checked directly against the reserved ranges. A DNS
	// name is left to Linear to resolve; we block only the names/literals a
	// caller can control locally.
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
			return fmt.Errorf("webhook URL host %q is not allowed (internal address)", host)
		}
	}
	return nil
}

// webhookLabel renders a webhook's label, falling back to "(no label)".
func webhookLabel(f api.WebhookFields) string {
	if f.Label != nil && *f.Label != "" {
		return *f.Label
	}
	return "(no label)"
}

// webhookURL renders a webhook's URL, falling back to "-" for the null URL
// Linear returns on webhooks whose target has been cleared.
func webhookURL(f api.WebhookFields) string {
	if f.Url != nil {
		return *f.Url
	}
	return "-"
}

func init() {
	rootCmd.AddCommand(webhookCmd)
	webhookCmd.AddCommand(webhookListCmd)
	webhookCmd.AddCommand(webhookGetCmd)
	webhookCmd.AddCommand(webhookCreateCmd)
	webhookCmd.AddCommand(webhookUpdateCmd)
	webhookCmd.AddCommand(webhookDeleteCmd)
	webhookCmd.AddCommand(webhookRotateSecretCmd)

	webhookListCmd.Flags().IntP("limit", "l", 50, "Maximum number of webhooks to fetch")

	webhookCreateCmd.Flags().String("url", "", "Target URL (required, must be public http/https)")
	webhookCreateCmd.Flags().StringSlice("resource-types", nil, "Resource types to subscribe to (repeatable or comma-separated, e.g. Issue,Comment)")
	webhookCreateCmd.Flags().String("label", "", "Human-readable label")
	webhookCreateCmd.Flags().String("secret", "", "Signing secret (Linear generates one if omitted)")
	webhookCreateCmd.Flags().Bool("enabled", true, "Whether the webhook is enabled")
	webhookCreateCmd.Flags().Bool("all-public-teams", false, "Deliver events for all public teams")
	webhookCreateCmd.Flags().StringP("team", "t", "", "Scope the webhook to a team (key, name, or ID)")

	webhookUpdateCmd.Flags().String("url", "", "New target URL (must be public http/https)")
	webhookUpdateCmd.Flags().StringSlice("resource-types", nil, "New resource types (replaces existing)")
	webhookUpdateCmd.Flags().String("label", "", "New label")
	webhookUpdateCmd.Flags().String("secret", "", "New signing secret")
	webhookUpdateCmd.Flags().Bool("enabled", true, "Whether the webhook is enabled")
}
