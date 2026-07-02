package cmd

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var userExternalCmd = &cobra.Command{
	Use:   "external",
	Short: "Inspect external users (read-only)",
	Long: `Inspect external users: people who interact with Linear through integrated
services (Slack, Jira, GitHub) without a Linear account.

External-user records carry PII (names, emails). --json prints them raw.

Examples:
  lincli user external list
  lincli user external get <id>`,
}

var userExternalListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List external users",
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

		resp, err := api.ListExternalUsers(ctx, client, limitPtr, nil)
		if err != nil {
			return fmt.Errorf("Failed to list external users: %v", err)
		}

		if resp.ExternalUsers == nil || len(resp.ExternalUsers.Nodes) == 0 {
			output.Info("No external users found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.ExternalUsers.Nodes)
			return nil
		}

		headers := []string{"ID", "Name", "Display Name", "Email", "Last Seen"}
		rows := make([][]string, len(resp.ExternalUsers.Nodes))
		for i, node := range resp.ExternalUsers.Nodes {
			f := node.ExternalUserFields
			lastSeen := ""
			if f.LastSeen != nil {
				lastSeen = f.LastSeen.Format("2006-01-02")
			}
			rows[i] = []string{
				f.Id,
				truncateString(f.Name, 25),
				truncateString(f.DisplayName, 20),
				derefStr(f.Email),
				lastSeen,
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d external users\n", color.New(color.FgGreen).Sprint("✓"), len(resp.ExternalUsers.Nodes))
		}
		return nil
	},
}

var userExternalGetCmd = &cobra.Command{
	Use:     "get <id>",
	Aliases: []string{"show"},
	Short:   "Get an external user",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		resp, err := api.GetExternalUser(context.Background(), client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to get external user: %v", err)
		}
		if resp.ExternalUser == nil {
			return fmt.Errorf("external user %q not found", args[0])
		}
		f := resp.ExternalUser.ExternalUserFields

		if jsonOut {
			output.JSON(resp.ExternalUser)
			return nil
		}

		if plaintext {
			fmt.Printf("# %s\n", f.Name)
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Display Name**: %s\n", f.DisplayName)
			fmt.Printf("- **Email**: %s\n", derefStr(f.Email))
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("External user:"), f.Name)
		fmt.Printf("  ID:           %s\n", f.Id)
		fmt.Printf("  Display Name: %s\n", f.DisplayName)
		fmt.Printf("  Email:        %s\n", derefStr(f.Email))
		if f.LastSeen != nil {
			fmt.Printf("  Last Seen:    %s\n", f.LastSeen.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

func init() {
	userCmd.AddCommand(userExternalCmd)
	userExternalCmd.AddCommand(userExternalListCmd)
	userExternalCmd.AddCommand(userExternalGetCmd)

	userExternalListCmd.Flags().IntP("limit", "l", 50, "Maximum number of external users to fetch")
}
