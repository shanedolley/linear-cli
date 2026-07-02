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

var orgCmd = &cobra.Command{
	Use:     "org",
	Aliases: []string{"organization"},
	Short:   "Manage your Linear organization",
	Long: `View and manage the workspace organization and its invites.

Examples:
  lincli org get                                  # Organization details
  lincli org update --name "Acme"                 # Update org settings
  lincli org invite list                          # List invites
  lincli org invite create --email a@b.com --role user
  lincli org invite resend INVITE-ID
  lincli org invite delete INVITE-ID`,
}

var orgGetCmd = &cobra.Command{
	Use:     "get",
	Aliases: []string{"show"},
	Short:   "Show organization details",
	Long:    `Show details about the current workspace organization.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)

		resp, err := api.GetOrganization(context.Background(), client)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get organization: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.Organization == nil {
			output.Error("Organization not found", plaintext, jsonOut)
			os.Exit(1)
		}
		o := resp.Organization.OrganizationFields

		if jsonOut {
			output.JSON(resp.Organization)
			return
		}

		if plaintext {
			fmt.Printf("# %s\n\n", o.Name)
			fmt.Printf("- **ID**: %s\n", o.Id)
			fmt.Printf("- **URL key**: %s\n", o.UrlKey)
			fmt.Printf("- **Users**: %d\n", o.UserCount)
			return
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Organization:"), o.Name)
		fmt.Printf("  ID:      %s\n", o.Id)
		fmt.Printf("  URL key: %s\n", o.UrlKey)
		fmt.Printf("  Users:   %d\n", o.UserCount)
	},
}

var orgUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update organization settings",
	Long:  `Update the organization's name or URL key.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)

		input := api.OrganizationUpdateInput{}
		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
		}
		if cmd.Flags().Changed("url-key") {
			urlKey, _ := cmd.Flags().GetString("url-key")
			input.UrlKey = &urlKey
		}

		if input.Name == nil && input.UrlKey == nil {
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.OrganizationUpdate(context.Background(), client, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update organization: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.OrganizationUpdate == nil || !resp.OrganizationUpdate.Success {
			output.Error("Failed to update organization", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.OrganizationUpdate.Organization)
		} else {
			output.Success("Updated organization", plaintext, jsonOut)
		}
	},
}

var orgInviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Manage organization invites",
	Long:  `List, create, delete, and resend organization invites.`,
}

var orgInviteListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List organization invites",
	Long:    `List pending and accepted invites for the workspace.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		resp, err := api.ListOrganizationInvites(context.Background(), client, limitPtr, nil)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list invites: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.OrganizationInvites == nil || len(resp.OrganizationInvites.Nodes) == 0 {
			output.Info("No invites found", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(resp.OrganizationInvites.Nodes)
			return
		}

		headers := []string{"ID", "Email", "Role", "Status"}
		rows := make([][]string, len(resp.OrganizationInvites.Nodes))
		for i, node := range resp.OrganizationInvites.Nodes {
			f := node.OrganizationInviteFields
			rows[i] = []string{
				f.Id,
				truncateString(f.Email, 35),
				string(f.Role),
				inviteStatus(f),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d invites\n", color.New(color.FgGreen).Sprint("✓"), len(resp.OrganizationInvites.Nodes))
		}
	},
}

var orgInviteCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create an organization invite",
	Long:    `Invite a user to the workspace by email. --role defaults to 'user'.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		email, _ := cmd.Flags().GetString("email")
		if email == "" {
			output.Error("Email is required (--email)", plaintext, jsonOut)
			os.Exit(1)
		}

		input := api.OrganizationInviteCreateInput{Email: email}
		if cmd.Flags().Changed("role") {
			roleStr, _ := cmd.Flags().GetString("role")
			role, err := validateUserRole(roleStr)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			input.Role = &role
		}

		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)

		resp, err := api.OrganizationInviteCreate(context.Background(), client, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create invite: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.OrganizationInviteCreate == nil || !resp.OrganizationInviteCreate.Success {
			output.Error("Failed to create invite", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.OrganizationInviteCreate.OrganizationInvite)
		} else {
			output.Success(fmt.Sprintf("Invited %s", email), plaintext, jsonOut)
		}
	},
}

var orgInviteDeleteCmd = &cobra.Command{
	Use:     "delete <invite-id>",
	Aliases: []string{"rm"},
	Short:   "Delete an organization invite",
	Long:    `Delete (revoke) a pending organization invite.`,
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

		resp, err := api.OrganizationInviteDelete(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete invite: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.OrganizationInviteDelete == nil || !resp.OrganizationInviteDelete.Success {
			output.Error("Failed to delete invite", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.OrganizationInviteDelete.EntityId})
		} else {
			output.Success(fmt.Sprintf("Deleted invite %s", args[0]), plaintext, jsonOut)
		}
	},
}

var orgInviteResendCmd = &cobra.Command{
	Use:   "resend <invite-id>",
	Short: "Resend an organization invite",
	Long:  `Resend a pending organization invite email.`,
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

		resp, err := api.ResendOrganizationInvite(context.Background(), client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to resend invite: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.ResendOrganizationInvite == nil || !resp.ResendOrganizationInvite.Success {
			output.Error("Failed to resend invite", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.ResendOrganizationInvite.EntityId})
		} else {
			output.Success(fmt.Sprintf("Resent invite %s", args[0]), plaintext, jsonOut)
		}
	},
}

// inviteStatus reports whether an invite has been accepted or is still pending.
func inviteStatus(f api.OrganizationInviteFields) string {
	if f.AcceptedAt != nil {
		return "accepted"
	}
	return "pending"
}

func init() {
	rootCmd.AddCommand(orgCmd)
	orgCmd.AddCommand(orgGetCmd)
	orgCmd.AddCommand(orgUpdateCmd)
	orgCmd.AddCommand(orgInviteCmd)
	orgInviteCmd.AddCommand(orgInviteListCmd)
	orgInviteCmd.AddCommand(orgInviteCreateCmd)
	orgInviteCmd.AddCommand(orgInviteDeleteCmd)
	orgInviteCmd.AddCommand(orgInviteResendCmd)

	orgUpdateCmd.Flags().String("name", "", "Organization name")
	orgUpdateCmd.Flags().String("url-key", "", "Organization URL key")

	orgInviteListCmd.Flags().IntP("limit", "l", 50, "Maximum number of invites to fetch")

	orgInviteCreateCmd.Flags().StringP("email", "e", "", "Email address to invite (required)")
	orgInviteCreateCmd.Flags().String("role", "", "Organization role: owner, admin, guest, user, or app (default user)")
}
