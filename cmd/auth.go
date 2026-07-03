package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Linear",
	Long: `Authenticate with Linear using Personal API Key.

Examples:
  lincli auth              # Interactive authentication
  lincli auth login        # Same as above
  lincli auth status       # Check authentication status
  lincli auth logout       # Clear stored credentials`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default behavior is to run login
		return loginCmd.RunE(cmd, args)
	},
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Linear",
	Long:  `Authenticate with Linear using Personal API Key.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		if !plaintext && !jsonOut {
			fmt.Println(color.New(color.FgCyan, color.Bold).Sprint("🔐 Linear Authentication"))
			fmt.Println()
		}

		err := auth.Login(plaintext, jsonOut)
		if err != nil {
			return fmt.Errorf("Authentication failed: %w", err)
		}

		if !plaintext && !jsonOut {
			fmt.Println(color.New(color.FgGreen).Sprint("✅ Successfully authenticated with Linear!"))
		} else if jsonOut {
			output.JSON(map[string]interface{}{
				"status":  "success",
				"message": "Successfully authenticated with Linear",
			})
		} else {
			fmt.Println("Successfully authenticated with Linear")
		}
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check authentication status",
	Long:  `Check if you are currently authenticated with Linear.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		user, err := auth.GetCurrentUser()
		if err != nil {
			if !plaintext && !jsonOut {
				fmt.Println(color.New(color.FgRed).Sprint("❌ Not authenticated"))
			} else if jsonOut {
				output.JSON(map[string]interface{}{
					"authenticated": false,
					"error":         err.Error(),
				})
			} else {
				fmt.Println("Not authenticated")
			}
			// Output already rendered above; errSilent exits non-zero quietly.
			return errSilent
		}

		if jsonOut {
			output.JSON(map[string]interface{}{
				"authenticated": true,
				"user":          user,
			})
		} else if plaintext {
			fmt.Printf("Authenticated as: %s (%s)\n", user.Name, user.Email)
		} else {
			fmt.Println(color.New(color.FgGreen).Sprint("✅ Authenticated"))
			fmt.Printf("User: %s\n", color.New(color.FgCyan).Sprint(user.Name))
			fmt.Printf("Email: %s\n", color.New(color.FgCyan).Sprint(user.Email))
		}
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from Linear",
	Long:  `Clear stored Linear credentials.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		err := auth.Logout()
		if err != nil {
			return fmt.Errorf("Logout failed: %w", err)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{
				"status":  "success",
				"message": "Successfully logged out",
			})
		} else if plaintext {
			fmt.Println("Successfully logged out")
		} else {
			fmt.Println(color.New(color.FgGreen).Sprint("✅ Successfully logged out"))
		}
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user",
	Long:  `Display information about the currently authenticated user.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusCmd.RunE(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(statusCmd)
	authCmd.AddCommand(logoutCmd)

	// Add whoami as a top-level command too
	rootCmd.AddCommand(whoamiCmd)
}
