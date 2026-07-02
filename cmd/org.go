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
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.GetOrganization(ctx, client)
		if err != nil {
			return fmt.Errorf("Failed to get organization: %v", err)
		}
		if resp.Organization == nil {
			return errors.New("Organization not found")
		}
		o := resp.Organization.OrganizationFields

		if jsonOut {
			output.JSON(resp.Organization)
			return nil
		}

		if plaintext {
			fmt.Printf("# %s\n\n", o.Name)
			fmt.Printf("- **ID**: %s\n", o.Id)
			fmt.Printf("- **URL key**: %s\n", o.UrlKey)
			fmt.Printf("- **Users**: %d\n", o.UserCount)
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Organization:"), o.Name)
		fmt.Printf("  ID:      %s\n", o.Id)
		fmt.Printf("  URL key: %s\n", o.UrlKey)
		fmt.Printf("  Users:   %d\n", o.UserCount)
		return nil
	},
}

var orgUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update organization settings",
	Long:  `Update the organization's name or URL key.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

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
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.OrganizationUpdate(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to update organization: %v", err)
		}
		if resp.OrganizationUpdate == nil || !resp.OrganizationUpdate.Success {
			return errors.New("Failed to update organization")
		}

		if jsonOut {
			output.JSON(resp.OrganizationUpdate.Organization)
		} else {
			output.Success("Updated organization", plaintext, jsonOut)
		}
		return nil
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

		resp, err := api.ListOrganizationInvites(ctx, client, limitPtr, nil)
		if err != nil {
			return fmt.Errorf("Failed to list invites: %v", err)
		}
		if resp.OrganizationInvites == nil || len(resp.OrganizationInvites.Nodes) == 0 {
			output.Info("No invites found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.OrganizationInvites.Nodes)
			return nil
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
		return nil
	},
}

var orgInviteCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create an organization invite",
	Long:    `Invite a user to the workspace by email. --role defaults to 'user'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		email, _ := cmd.Flags().GetString("email")
		if email == "" {
			return errors.New("Email is required (--email)")
		}

		input := api.OrganizationInviteCreateInput{Email: email}
		if cmd.Flags().Changed("role") {
			roleStr, _ := cmd.Flags().GetString("role")
			role, err := validateUserRole(roleStr)
			if err != nil {
				return err
			}
			input.Role = &role
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.OrganizationInviteCreate(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to create invite: %v", err)
		}
		if resp.OrganizationInviteCreate == nil || !resp.OrganizationInviteCreate.Success {
			return errors.New("Failed to create invite")
		}

		if jsonOut {
			output.JSON(resp.OrganizationInviteCreate.OrganizationInvite)
		} else {
			output.Success(fmt.Sprintf("Invited %s", email), plaintext, jsonOut)
		}
		return nil
	},
}

var orgInviteDeleteCmd = &cobra.Command{
	Use:     "delete <invite-id>",
	Aliases: []string{"rm"},
	Short:   "Delete an organization invite",
	Long:    `Delete (revoke) a pending organization invite.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.OrganizationInviteDelete(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete invite: %v", err)
		}
		if resp.OrganizationInviteDelete == nil || !resp.OrganizationInviteDelete.Success {
			return errors.New("Failed to delete invite")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.OrganizationInviteDelete.EntityId})
		} else {
			output.Success(fmt.Sprintf("Deleted invite %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var orgInviteResendCmd = &cobra.Command{
	Use:   "resend <invite-id>",
	Short: "Resend an organization invite",
	Long:  `Resend a pending organization invite email.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.ResendOrganizationInvite(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to resend invite: %v", err)
		}
		if resp.ResendOrganizationInvite == nil || !resp.ResendOrganizationInvite.Success {
			return errors.New("Failed to resend invite")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": resp.ResendOrganizationInvite.EntityId})
		} else {
			output.Success(fmt.Sprintf("Resent invite %s", args[0]), plaintext, jsonOut)
		}
		return nil
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
