package cmd

import (
	"context"
	"errors"
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

// userCmd represents the user command
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage Linear users",
	Long: `Manage Linear users including listing users, viewing user details, and showing the current user.

Examples:
  lincli user list              # List all users
  lincli user get john@example.com  # Get user details
  lincli user me                # Show current user`,
}

var userListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List users",
	Long:    `List all users in your Linear workspace.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		// Get filters
		limit, _ := cmd.Flags().GetInt("limit")
		activeOnly, _ := cmd.Flags().GetBool("active")

		// Get sort option
		sortBy, _ := cmd.Flags().GetString("sort")
		var orderByEnum *api.PaginationOrderBy
		if sortBy != "" {
			switch sortBy {
			case "created", "createdAt":
				val := api.PaginationOrderByCreatedat
				orderByEnum = &val
			case "updated", "updatedAt":
				val := api.PaginationOrderByUpdatedat
				orderByEnum = &val
			case "linear":
				// Use nil for Linear's default sort
				orderByEnum = nil
			default:
				return fmt.Errorf("Invalid sort option: %s. Valid options are: linear, created, updated", sortBy)
			}
		}

		// Convert limit to pointer
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		// Get users
		resp, err := api.ListUsers(context.Background(), client, limitPtr, nil, orderByEnum)
		if err != nil {
			return fmt.Errorf("Failed to list users: %v", err)
		}

		// Filter active users if requested
		filteredUsers := resp.Users.Nodes
		if activeOnly {
			activeUsers := resp.Users.Nodes[:0] // reuse slice with zero length
			for _, user := range resp.Users.Nodes {
				if user.UserListFields.Active {
					activeUsers = append(activeUsers, user)
				}
			}
			filteredUsers = activeUsers
		}

		// Handle output
		if jsonOut {
			// Extract just the fields for JSON output
			var jsonUsers []api.UserListFields
			for _, user := range filteredUsers {
				jsonUsers = append(jsonUsers, user.UserListFields)
			}
			output.JSON(jsonUsers)
		} else if plaintext {
			fmt.Println("Name\tEmail\tRole\tActive")
			for _, user := range filteredUsers {
				f := user.UserListFields
				role := "Member"
				if f.Admin {
					role = "Admin"
				}
				fmt.Printf("%s\t%s\t%s\t%v\n",
					f.Name,
					f.Email,
					role,
					f.Active,
				)
			}
		} else {
			// Table output
			headers := []string{"Name", "Email", "Role", "Status"}
			rows := [][]string{}

			for _, user := range filteredUsers {
				f := user.UserListFields
				role := "Member"
				roleColor := color.New(color.FgWhite)
				if f.Admin {
					role = "Admin"
					roleColor = color.New(color.FgYellow)
				}
				if f.IsMe {
					role = role + " (You)"
					roleColor = color.New(color.FgCyan, color.Bold)
				}

				status := color.New(color.FgGreen).Sprint("✓ Active")
				if !f.Active {
					status = color.New(color.FgRed).Sprint("✗ Inactive")
				}

				rows = append(rows, []string{
					f.Name,
					color.New(color.FgCyan).Sprint(f.Email),
					roleColor.Sprint(role),
					status,
				})
			}

			output.Table(output.TableData{
				Headers: headers,
				Rows:    rows,
			}, plaintext, jsonOut)

			if !plaintext && !jsonOut {
				fmt.Printf("\n%s %d users\n",
					color.New(color.FgGreen).Sprint("✓"),
					len(filteredUsers))
			}
		}
		return nil
	},
}

var userGetCmd = &cobra.Command{
	Use:     "get EMAIL",
	Aliases: []string{"show"},
	Short:   "Get user details",
	Long:    `Get detailed information about a specific user by email.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		email := args[0]

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		// Get user details using generated function
		filter := &api.UserFilter{
			Email: &api.StringComparator{Eq: &email},
		}

		userResp, err := api.GetUserByEmail(context.Background(), client, filter)
		if err != nil {
			return fmt.Errorf("Failed to get user: %v", err)
		}

		if len(userResp.Users.Nodes) == 0 {
			return fmt.Errorf("User not found with email: %s", email)
		}

		user := &userResp.Users.Nodes[0].UserDetailFields

		// Handle output
		if jsonOut {
			output.JSON(user)
		} else if plaintext {
			fmt.Printf("ID: %s\n", user.Id)
			fmt.Printf("Name: %s\n", user.Name)
			fmt.Printf("Email: %s\n", user.Email)
			fmt.Printf("Admin: %v\n", user.Admin)
			fmt.Printf("Active: %v\n", user.Active)
			if user.AvatarUrl != nil && *user.AvatarUrl != "" {
				fmt.Printf("Avatar: %s\n", *user.AvatarUrl)
			}
		} else {
			// Formatted output
			fmt.Println()
			fmt.Printf("%s %s\n",
				color.New(color.FgCyan, color.Bold).Sprint("👤 User:"),
				user.Name)
			fmt.Println(strings.Repeat("─", 50))

			fmt.Printf("\n%s %s\n", color.New(color.Bold).Sprint("Email:"),
				color.New(color.FgCyan).Sprint(user.Email))
			fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("ID:"), user.Id)

			role := "Member"
			roleColor := color.New(color.FgWhite)
			if user.Admin {
				role = "Admin"
				roleColor = color.New(color.FgYellow)
			}
			if user.IsMe {
				role = role + " (You)"
				roleColor = color.New(color.FgCyan, color.Bold)
			}
			fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Role:"), roleColor.Sprint(role))

			status := color.New(color.FgGreen).Sprint("✓ Active")
			if !user.Active {
				status = color.New(color.FgRed).Sprint("✗ Inactive")
			}
			fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Status:"), status)

			if user.AvatarUrl != nil && *user.AvatarUrl != "" {
				fmt.Printf("\n%s\n%s\n", color.New(color.Bold).Sprint("Avatar:"),
					color.New(color.FgBlue).Sprint(*user.AvatarUrl))
			}
			fmt.Println()
		}
		return nil
	},
}

var userMeCmd = &cobra.Command{
	Use:   "me",
	Short: "Show current user",
	Long:  `Display information about the currently authenticated user.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}

		// Get current user
		resp, err := api.GetViewer(context.Background(), client)
		if err != nil {
			return fmt.Errorf("Failed to get current user: %v", err)
		}
		user := resp.Viewer.UserDetailFields

		// Handle output
		if jsonOut {
			output.JSON(user)
		} else if plaintext {
			fmt.Printf("ID: %s\n", user.Id)
			fmt.Printf("Name: %s\n", user.Name)
			fmt.Printf("Email: %s\n", user.Email)
			fmt.Printf("Admin: %v\n", user.Admin)
			fmt.Printf("Active: %v\n", user.Active)
			if user.AvatarUrl != nil && *user.AvatarUrl != "" {
				fmt.Printf("Avatar: %s\n", *user.AvatarUrl)
			}
		} else {
			// Formatted output
			fmt.Println()
			fmt.Printf("%s %s\n",
				color.New(color.FgCyan, color.Bold).Sprint("👤 Current User:"),
				user.Name)
			fmt.Println(strings.Repeat("─", 50))

			fmt.Printf("\n%s %s\n", color.New(color.Bold).Sprint("Email:"),
				color.New(color.FgCyan).Sprint(user.Email))
			fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("ID:"), user.Id)

			role := "Member"
			roleColor := color.New(color.FgWhite)
			if user.Admin {
				role = "Admin"
				roleColor = color.New(color.FgYellow, color.Bold)
			}
			fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Role:"), roleColor.Sprint(role))

			status := color.New(color.FgGreen).Sprint("✓ Active")
			if !user.Active {
				status = color.New(color.FgRed).Sprint("✗ Inactive")
			}
			fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("Status:"), status)

			if user.AvatarUrl != nil && *user.AvatarUrl != "" {
				fmt.Printf("\n%s\n%s\n", color.New(color.Bold).Sprint("Avatar:"),
					color.New(color.FgBlue).Sprint(*user.AvatarUrl))
			}
			fmt.Println()
		}
		return nil
	},
}

var userUpdateCmd = &cobra.Command{
	Use:   "update <email>",
	Short: "Update a user's profile",
	Long:  `Update a user's profile fields (name, display name, title, description, timezone).`,
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

		userID, err := resolveUser(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		input := api.UserUpdateInput{}
		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
		}
		if cmd.Flags().Changed("display-name") {
			displayName, _ := cmd.Flags().GetString("display-name")
			input.DisplayName = &displayName
		}
		if cmd.Flags().Changed("title") {
			title, _ := cmd.Flags().GetString("title")
			input.Title = &title
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
		}
		if cmd.Flags().Changed("timezone") {
			timezone, _ := cmd.Flags().GetString("timezone")
			input.Timezone = &timezone
		}

		if input.Name == nil && input.DisplayName == nil && input.Title == nil && input.Description == nil && input.Timezone == nil {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.UserUpdate(ctx, client, userID, &input)
		if err != nil {
			return fmt.Errorf("Failed to update user: %v", err)
		}
		if resp.UserUpdate == nil || !resp.UserUpdate.Success {
			return errors.New("Failed to update user")
		}

		if jsonOut {
			output.JSON(resp.UserUpdate.User)
		} else {
			output.Success(fmt.Sprintf("Updated user %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var userSetRoleCmd = &cobra.Command{
	Use:   "set-role <email>",
	Short: "Set a user's organization role",
	Long: `Set a user's ORGANIZATION role: owner, admin, guest, user, or app. This is
the workspace-wide role, not team membership; use 'team set-role' to change a
user's role within a team.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		roleStr, _ := cmd.Flags().GetString("role")
		role, err := validateUserRole(roleStr)
		if err != nil {
			return err
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		userID, err := resolveUser(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		resp, err := api.UserChangeRole(ctx, client, userID, role)
		if err != nil {
			return fmt.Errorf("Failed to set role: %v", err)
		}
		if resp.UserChangeRole == nil || !resp.UserChangeRole.Success {
			return errors.New("Failed to set role")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "email": args[0], "role": string(role)})
		} else {
			output.Success(fmt.Sprintf("Set %s org role to %s", args[0], role), plaintext, jsonOut)
		}
		return nil
	},
}

var userSuspendCmd = &cobra.Command{
	Use:   "suspend <email>",
	Short: "Suspend a user",
	Long: `Suspend a user, deactivating their account and revoking workspace access.
This action executes immediately with no confirmation prompt. Reversible with
'user unsuspend'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runUserSuspension(cmd, args[0], true)
		return nil
	},
}

var userUnsuspendCmd = &cobra.Command{
	Use:   "unsuspend <email>",
	Short: "Unsuspend a user",
	Long:  `Reactivate a suspended user, restoring their workspace access.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runUserSuspension(cmd, args[0], false)
		return nil
	},
}

// runUserSuspension handles suspend/unsuspend, which share the same shape:
// resolve the user, call the mutation, report.
func runUserSuspension(cmd *cobra.Command, ref string, suspend bool) {
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

	userID, err := resolveUser(ctx, client, cache, ref)
	if err != nil {
		output.Error(err.Error(), plaintext, jsonOut)
		os.Exit(1)
	}

	verb, pastVerb := "suspend", "Suspended"
	if !suspend {
		verb, pastVerb = "unsuspend", "Unsuspended"
	}

	var success bool
	if suspend {
		resp, err := api.UserSuspend(ctx, client, userID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to suspend user: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		success = resp.UserSuspend != nil && resp.UserSuspend.Success
	} else {
		resp, err := api.UserUnsuspend(ctx, client, userID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to unsuspend user: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		success = resp.UserUnsuspend != nil && resp.UserUnsuspend.Success
	}

	if !success {
		output.Error(fmt.Sprintf("Failed to %s user", verb), plaintext, jsonOut)
		os.Exit(1)
	}

	if jsonOut {
		output.JSON(map[string]interface{}{"success": true, "email": ref})
	} else {
		output.Success(fmt.Sprintf("%s user %s", pastVerb, ref), plaintext, jsonOut)
	}
}

var userSettingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Manage your user settings",
	Long:  `Manage the authenticated user's settings.`,
}

var userSettingsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update your email subscription settings",
	Long: `Update the authenticated user's email subscription settings. Linear only
exposes the current user's own settings, so this always targets you.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		input := api.UserSettingsUpdateInput{}
		if cmd.Flags().Changed("changelog") {
			v, _ := cmd.Flags().GetBool("changelog")
			input.SubscribedToChangelog = &v
		}
		if cmd.Flags().Changed("marketing") {
			v, _ := cmd.Flags().GetBool("marketing")
			input.SubscribedToGeneralMarketingCommunications = &v
		}
		if cmd.Flags().Changed("invite-accepted") {
			v, _ := cmd.Flags().GetBool("invite-accepted")
			input.SubscribedToInviteAccepted = &v
		}
		if cmd.Flags().Changed("dpa") {
			v, _ := cmd.Flags().GetBool("dpa")
			input.SubscribedToDPA = &v
		}
		if cmd.Flags().Changed("privacy-legal") {
			v, _ := cmd.Flags().GetBool("privacy-legal")
			input.SubscribedToPrivacyLegalUpdates = &v
		}

		if input.SubscribedToChangelog == nil && input.SubscribedToGeneralMarketingCommunications == nil &&
			input.SubscribedToInviteAccepted == nil && input.SubscribedToDPA == nil &&
			input.SubscribedToPrivacyLegalUpdates == nil {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		settingsResp, err := api.GetUserSettings(ctx, client)
		if err != nil {
			return fmt.Errorf("Failed to load your settings: %v", err)
		}
		if settingsResp.UserSettings == nil {
			return errors.New("Could not resolve your settings")
		}

		resp, err := api.UserSettingsUpdate(ctx, client, settingsResp.UserSettings.Id, &input)
		if err != nil {
			return fmt.Errorf("Failed to update settings: %v", err)
		}
		if resp.UserSettingsUpdate == nil || !resp.UserSettingsUpdate.Success {
			return errors.New("Failed to update settings")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true})
		} else {
			output.Success("Updated your settings", plaintext, jsonOut)
		}
		return nil
	},
}

// validateUserRole checks an organization role string against Linear's
// UserRoleType values. "member" is a team-scope concept and is rejected here.
func validateUserRole(role string) (api.UserRoleType, error) {
	switch strings.ToLower(role) {
	case "owner", "admin", "guest", "user", "app":
		return api.UserRoleType(strings.ToLower(role)), nil
	default:
		return "", fmt.Errorf("invalid org role %q. Valid values: owner, admin, guest, user, app", role)
	}
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userGetCmd)
	userCmd.AddCommand(userMeCmd)
	userCmd.AddCommand(userUpdateCmd)
	userCmd.AddCommand(userSetRoleCmd)
	userCmd.AddCommand(userSuspendCmd)
	userCmd.AddCommand(userUnsuspendCmd)
	userCmd.AddCommand(userSettingsCmd)
	userSettingsCmd.AddCommand(userSettingsUpdateCmd)

	// List command flags
	userListCmd.Flags().IntP("limit", "l", 50, "Maximum number of users to return")
	userListCmd.Flags().BoolP("active", "a", false, "Show only active users")
	userListCmd.Flags().StringP("sort", "o", "linear", "Sort order: linear (default), created, updated")

	// Update command flags
	userUpdateCmd.Flags().String("name", "", "Full name")
	userUpdateCmd.Flags().String("display-name", "", "Display name")
	userUpdateCmd.Flags().String("title", "", "Job title")
	userUpdateCmd.Flags().StringP("description", "d", "", "Profile description")
	userUpdateCmd.Flags().String("timezone", "", "Timezone (e.g. America/New_York)")

	// set-role flags
	userSetRoleCmd.Flags().String("role", "", "Organization role: owner, admin, guest, user, or app (required)")
	_ = userSetRoleCmd.MarkFlagRequired("role")

	// settings update flags
	userSettingsUpdateCmd.Flags().Bool("changelog", false, "Subscribe to changelog emails")
	userSettingsUpdateCmd.Flags().Bool("marketing", false, "Subscribe to general marketing communications")
	userSettingsUpdateCmd.Flags().Bool("invite-accepted", false, "Subscribe to invite-accepted emails")
	userSettingsUpdateCmd.Flags().Bool("dpa", false, "Subscribe to data-processing-agreement emails")
	userSettingsUpdateCmd.Flags().Bool("privacy-legal", false, "Subscribe to privacy and legal update emails")
}
