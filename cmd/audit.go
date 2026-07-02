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

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Inspect the workspace audit log",
	Long: `Inspect the workspace audit log: security-relevant events such as logins,
invites, exports, and setting changes.

Audit entries carry PII (actor names, emails, IP addresses). --json prints the
raw records.

Examples:
  lincli audit list
  lincli audit list --type login --limit 20
  lincli audit types`,
}

var auditListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List audit log entries",
	Long: `List recent audit log entries, newest first. Filter by event type with
--type (see 'lincli audit types' for the catalog).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		var filter *api.AuditEntryFilter
		if cmd.Flags().Changed("type") {
			t, _ := cmd.Flags().GetString("type")
			filter = &api.AuditEntryFilter{Type: &api.StringComparator{Eq: &t}}
		}

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		resp, err := api.ListAuditEntries(ctx, client, filter, limitPtr, nil)
		if err != nil {
			return fmt.Errorf("Failed to list audit entries: %v", err)
		}

		if resp.AuditEntries == nil || len(resp.AuditEntries.Nodes) == 0 {
			output.Info("No audit entries found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.AuditEntries.Nodes)
			return nil
		}

		headers := []string{"Time", "Type", "Actor", "IP", "Country"}
		rows := make([][]string, len(resp.AuditEntries.Nodes))
		for i, node := range resp.AuditEntries.Nodes {
			f := node.AuditEntryFields
			actor := ""
			if f.Actor != nil {
				actor = f.Actor.Name
			}
			rows[i] = []string{
				f.CreatedAt.Format("2006-01-02 15:04"),
				f.Type,
				actor,
				derefStr(f.Ip),
				derefStr(f.CountryCode),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d entries\n", color.New(color.FgGreen).Sprint("✓"), len(resp.AuditEntries.Nodes))
		}
		return nil
	},
}

var auditTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List the audit event-type catalog",
	Long:  `List every audit event type and its description, for use with 'audit list --type'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		resp, err := api.GetAuditEntryTypes(context.Background(), client)
		if err != nil {
			return fmt.Errorf("Failed to get audit entry types: %v", err)
		}

		if len(resp.AuditEntryTypes) == 0 {
			output.Info("No audit entry types found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.AuditEntryTypes)
			return nil
		}

		headers := []string{"Type", "Description"}
		rows := make([][]string, len(resp.AuditEntryTypes))
		for i, t := range resp.AuditEntryTypes {
			rows[i] = []string{t.Type, t.Description}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.AddCommand(auditListCmd)
	auditCmd.AddCommand(auditTypesCmd)

	auditListCmd.Flags().IntP("limit", "l", 50, "Maximum number of entries to fetch")
	auditListCmd.Flags().String("type", "", "Filter by audit event type (see 'audit types')")
}
