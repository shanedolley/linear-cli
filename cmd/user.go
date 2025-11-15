package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/fatih/color"
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
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		// Get auth header
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		// Create API client
		client := api.NewClient(authHeader)

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
				output.Error(fmt.Sprintf("Invalid sort option: %s. Valid options are: linear, created, updated", sortBy), plaintext, jsonOut)
				os.Exit(1)
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
			output.Error(fmt.Sprintf("Failed to list users: %v", err), plaintext, jsonOut)
			os.Exit(1)
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
	},
}

var userGetCmd = &cobra.Command{
	Use:     "get EMAIL",
	Aliases: []string{"show"},
	Short:   "Get user details",
	Long:    `Get detailed information about a specific user by email.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		email := args[0]

		// Get auth header
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		// Create API client
		client := api.NewClient(authHeader)

		// Get user details using generated function
		filter := &api.UserFilter{
			Email: &api.StringComparator{Eq: &email},
		}

		userResp, err := api.GetUserByEmail(context.Background(), client, filter)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get user: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if len(userResp.Users.Nodes) == 0 {
			output.Error(fmt.Sprintf("User not found with email: %s", email), plaintext, jsonOut)
			os.Exit(1)
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
	},
}

var userMeCmd = &cobra.Command{
	Use:   "me",
	Short: "Show current user",
	Long:  `Display information about the currently authenticated user.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		// Get auth header
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		// Create API client
		client := api.NewClient(authHeader)

		// Get current user
		resp, err := api.GetViewer(context.Background(), client)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get current user: %v", err), plaintext, jsonOut)
			os.Exit(1)
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
	},
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userGetCmd)
	userCmd.AddCommand(userMeCmd)

	// List command flags
	userListCmd.Flags().IntP("limit", "l", 50, "Maximum number of users to return")
	userListCmd.Flags().BoolP("active", "a", false, "Show only active users")
	userListCmd.Flags().StringP("sort", "o", "linear", "Sort order: linear (default), created, updated")
}
