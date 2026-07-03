package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var issueRelateCmd = &cobra.Command{
	Use:   "relate",
	Short: "Manage issue relations",
	Long: `Manage relations between issues.

Create and remove relations with 'issue link'. 'issue relate update' changes an
existing relation: its type, source issue, or related issue. Get the relation
ID from the Relations list in 'issue get <id> --json'.

Examples:
  lincli issue relate update <relation-id> --relation blocks`,
}

var issueRelateUpdateCmd = &cobra.Command{
	Use:   "update <relation-id>",
	Short: "Update an existing issue relation",
	Long: `Update an existing issue relation. Pass at least one of --relation (the
relation type: blocks, duplicate, related, similar), --issue, or --related (an
issue identifier such as TEAM-123 or a UUID).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		input := &api.IssueRelationUpdateInput{}
		changed := false

		if cmd.Flags().Changed("relation") {
			raw, _ := cmd.Flags().GetString("relation")
			relation, err := validateIssueRelationType(raw)
			if err != nil {
				return err
			}
			input.Type = &relation
			changed = true
		}
		if cmd.Flags().Changed("issue") {
			issue, _ := cmd.Flags().GetString("issue")
			input.IssueId = &issue
			changed = true
		}
		if cmd.Flags().Changed("related") {
			related, _ := cmd.Flags().GetString("related")
			input.RelatedIssueId = &related
			changed = true
		}

		if !changed {
			return errors.New("No updates specified. Use --relation, --issue, or --related.")
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		resp, err := api.UpdateIssueRelation(context.Background(), client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to update relation: %w", err)
		}
		if resp.IssueRelationUpdate == nil || !resp.IssueRelationUpdate.Success {
			return errors.New("Failed to update relation")
		}

		if jsonOut {
			output.JSON(resp.IssueRelationUpdate.IssueRelation)
		} else {
			output.Success(fmt.Sprintf("Updated relation %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// validateIssueRelationType normalizes and validates an issue relation type
// against the IssueRelationType values (blocks, duplicate, related, similar),
// matched case-insensitively and returned lowercase. An empty or unrecognized
// value returns an error listing the valid types.
func validateIssueRelationType(s string) (string, error) {
	switch v := strings.ToLower(strings.TrimSpace(s)); v {
	case "blocks", "duplicate", "related", "similar":
		return v, nil
	default:
		return "", fmt.Errorf("invalid relation %q: must be one of blocks, duplicate, related, similar", s)
	}
}

func init() {
	issueCmd.AddCommand(issueRelateCmd)
	issueRelateCmd.AddCommand(issueRelateUpdateCmd)

	issueRelateUpdateCmd.Flags().String("relation", "", "New relation type: blocks, duplicate, related, similar")
	issueRelateUpdateCmd.Flags().String("issue", "", "New source issue (identifier or UUID)")
	issueRelateUpdateCmd.Flags().String("related", "", "New related issue (identifier or UUID)")
}
