package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var gitCmd = &cobra.Command{
	Use:   "git",
	Short: "Manage Git automation rules",
	Long: `Manage a team's Git automation: automation states (move linked issues to a
workflow state on a Git event) and target branches (scope rules to branches).

These commands are write-only (create/update/delete): Linear's API does not
expose a query for automation rules, so capture the ID from --json output when
you create one.

Examples:
  lincli git target-branch create --team ENG --branch "release/.*" --regex
  lincli git state create --team ENG --event merge --state Done
  lincli git state delete <id>`,
}

// --- git state ---

var gitStateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage Git automation states",
	Long: `Manage Git automation states. A state transitions linked issues to a workflow
state when a Git event fires. The event is one of: draft, start, review,
mergeable, merge.`,
}

var gitStateCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a Git automation state",
	Long: `Create a Git automation state for a team. --team and --event are required.
Pass --state to set the target workflow state (omit it to take no action,
overriding any default rule for the event). Pass --target-branch to scope the
rule to a target branch (omit it to apply to all branches).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		team, _ := cmd.Flags().GetString("team")
		if team == "" {
			return errors.New("Team is required (--team)")
		}
		eventRaw, _ := cmd.Flags().GetString("event")
		event, err := validateGitEvent(eventRaw)
		if err != nil {
			return err
		}

		client, ctx, err := gitClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		teamID, err := resolveTeam(ctx, client, cache, team)
		if err != nil {
			return err
		}

		input := &api.GitAutomationStateCreateInput{
			TeamId: teamID,
			Event:  event,
		}
		if cmd.Flags().Changed("state") {
			state, _ := cmd.Flags().GetString("state")
			stateID, err := resolveWorkflowState(ctx, client, cache, teamID, state)
			if err != nil {
				return err
			}
			input.StateId = &stateID
		}
		if cmd.Flags().Changed("target-branch") {
			targetBranch, _ := cmd.Flags().GetString("target-branch")
			input.TargetBranchId = &targetBranch
		}

		resp, err := api.GitAutomationStateCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to create Git automation state: %v", err)
		}
		if resp.GitAutomationStateCreate == nil || !resp.GitAutomationStateCreate.Success {
			return errors.New("Failed to create Git automation state")
		}

		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		if jsonOut {
			output.JSON(resp.GitAutomationStateCreate.GitAutomationState)
		} else {
			output.Success(fmt.Sprintf("Created Git automation state %s", resp.GitAutomationStateCreate.GitAutomationState.GitAutomationStateFields.Id), plaintext, jsonOut)
		}
		return nil
	},
}

var gitStateUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a Git automation state",
	Long: `Update a Git automation state. Only the flags you pass are changed; omitted
fields keep their current value (verified: gitAutomationStateUpdate treats an
unset field as no-change, not as clear). Pass --team when --state is a state
name rather than an ID, so the name can be resolved within that team.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := gitClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		input := &api.GitAutomationStateUpdateInput{}
		changed := false

		if cmd.Flags().Changed("event") {
			eventRaw, _ := cmd.Flags().GetString("event")
			event, err := validateGitEvent(eventRaw)
			if err != nil {
				return err
			}
			input.Event = &event
			changed = true
		}
		if cmd.Flags().Changed("state") {
			state, _ := cmd.Flags().GetString("state")
			team, _ := cmd.Flags().GetString("team")
			if !isUUID(state) && team == "" {
				return errors.New("Pass --team to resolve a state name, or give --state as an ID")
			}
			stateID, err := resolveWorkflowState(ctx, client, cache, team, state)
			if err != nil {
				return err
			}
			input.StateId = &stateID
			changed = true
		}
		if cmd.Flags().Changed("target-branch") {
			targetBranch, _ := cmd.Flags().GetString("target-branch")
			input.TargetBranchId = &targetBranch
			changed = true
		}

		if !changed {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.GitAutomationStateUpdate(ctx, client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to update Git automation state: %v", err)
		}
		if resp.GitAutomationStateUpdate == nil || !resp.GitAutomationStateUpdate.Success {
			return errors.New("Failed to update Git automation state")
		}

		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		if jsonOut {
			output.JSON(resp.GitAutomationStateUpdate.GitAutomationState)
		} else {
			output.Success(fmt.Sprintf("Updated Git automation state %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var gitStateDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a Git automation state",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := gitClient()
		if err != nil {
			return err
		}

		resp, err := api.GitAutomationStateDelete(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete Git automation state: %v", err)
		}
		if resp.GitAutomationStateDelete == nil || !resp.GitAutomationStateDelete.Success {
			return errors.New("Failed to delete Git automation state")
		}

		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted Git automation state %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// --- git target-branch ---

var gitTargetBranchCmd = &cobra.Command{
	Use:     "target-branch",
	Aliases: []string{"branch"},
	Short:   "Manage Git automation target branches",
	Long: `Manage Git automation target branches. A target branch scopes automation rules
to specific branches, matched by exact name or by regular expression (--regex).`,
}

var gitTargetBranchCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a Git automation target branch",
	Long:    `Create a Git automation target branch. --team and --branch are required.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		team, _ := cmd.Flags().GetString("team")
		if team == "" {
			return errors.New("Team is required (--team)")
		}
		branch, _ := cmd.Flags().GetString("branch")
		if branch == "" {
			return errors.New("Branch pattern is required (--branch)")
		}

		client, ctx, err := gitClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		teamID, err := resolveTeam(ctx, client, cache, team)
		if err != nil {
			return err
		}

		isRegex, _ := cmd.Flags().GetBool("regex")
		input := &api.GitAutomationTargetBranchCreateInput{
			TeamId:        teamID,
			BranchPattern: branch,
			IsRegex:       &isRegex,
		}

		resp, err := api.GitAutomationTargetBranchCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to create target branch: %v", err)
		}
		if resp.GitAutomationTargetBranchCreate == nil || !resp.GitAutomationTargetBranchCreate.Success {
			return errors.New("Failed to create target branch")
		}

		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		if jsonOut {
			output.JSON(resp.GitAutomationTargetBranchCreate.TargetBranch)
		} else {
			output.Success(fmt.Sprintf("Created target branch %s", resp.GitAutomationTargetBranchCreate.TargetBranch.GitAutomationTargetBranchFields.Id), plaintext, jsonOut)
		}
		return nil
	},
}

var gitTargetBranchUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a Git automation target branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := gitClient()
		if err != nil {
			return err
		}

		input := &api.GitAutomationTargetBranchUpdateInput{}
		changed := false

		if cmd.Flags().Changed("branch") {
			branch, _ := cmd.Flags().GetString("branch")
			input.BranchPattern = &branch
			changed = true
		}
		if cmd.Flags().Changed("regex") {
			isRegex, _ := cmd.Flags().GetBool("regex")
			input.IsRegex = &isRegex
			changed = true
		}

		if !changed {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.GitAutomationTargetBranchUpdate(ctx, client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to update target branch: %v", err)
		}
		if resp.GitAutomationTargetBranchUpdate == nil || !resp.GitAutomationTargetBranchUpdate.Success {
			return errors.New("Failed to update target branch")
		}

		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		if jsonOut {
			output.JSON(resp.GitAutomationTargetBranchUpdate.TargetBranch)
		} else {
			output.Success(fmt.Sprintf("Updated target branch %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var gitTargetBranchDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a Git automation target branch",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, err := gitClient()
		if err != nil {
			return err
		}

		resp, err := api.GitAutomationTargetBranchDelete(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete target branch: %v", err)
		}
		if resp.GitAutomationTargetBranchDelete == nil || !resp.GitAutomationTargetBranchDelete.Success {
			return errors.New("Failed to delete target branch")
		}

		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted target branch %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// validateGitEvent normalizes and validates a Git automation event against the
// GitAutomationStates enum (draft | start | review | mergeable | merge),
// returning the typed enum value. An empty or unrecognized value returns an
// error listing the valid events.
func validateGitEvent(event string) (api.GitAutomationStates, error) {
	switch strings.ToLower(strings.TrimSpace(event)) {
	case "draft":
		return api.GitAutomationStatesDraft, nil
	case "start":
		return api.GitAutomationStatesStart, nil
	case "review":
		return api.GitAutomationStatesReview, nil
	case "mergeable":
		return api.GitAutomationStatesMergeable, nil
	case "merge":
		return api.GitAutomationStatesMerge, nil
	default:
		return "", fmt.Errorf("invalid event %q: must be one of draft, start, review, mergeable, merge", event)
	}
}

// gitClient builds the authenticated client and context shared by the git
// subcommands.
func gitClient() (graphql.Client, context.Context, error) {
	client, err := newGraphQLClient()
	if err != nil {
		return nil, nil, err
	}
	return client, context.Background(), nil
}

func init() {
	rootCmd.AddCommand(gitCmd)

	gitCmd.AddCommand(gitStateCmd)
	gitCmd.AddCommand(gitTargetBranchCmd)

	gitStateCmd.AddCommand(gitStateCreateCmd)
	gitStateCmd.AddCommand(gitStateUpdateCmd)
	gitStateCmd.AddCommand(gitStateDeleteCmd)

	gitTargetBranchCmd.AddCommand(gitTargetBranchCreateCmd)
	gitTargetBranchCmd.AddCommand(gitTargetBranchUpdateCmd)
	gitTargetBranchCmd.AddCommand(gitTargetBranchDeleteCmd)

	// git state
	gitStateCreateCmd.Flags().String("team", "", "Team key or ID (required)")
	gitStateCreateCmd.Flags().String("event", "", "Trigger event: draft, start, review, mergeable, merge (required)")
	gitStateCreateCmd.Flags().String("state", "", "Target workflow state name or ID (omit to take no action)")
	gitStateCreateCmd.Flags().String("target-branch", "", "Target branch ID to scope the rule to (omit for all branches)")

	gitStateUpdateCmd.Flags().String("event", "", "New trigger event: draft, start, review, mergeable, merge")
	gitStateUpdateCmd.Flags().String("state", "", "New target workflow state name or ID")
	gitStateUpdateCmd.Flags().String("team", "", "Team key or ID (needed to resolve a --state name)")
	gitStateUpdateCmd.Flags().String("target-branch", "", "New target branch ID")

	// git target-branch
	gitTargetBranchCreateCmd.Flags().String("team", "", "Team key or ID (required)")
	gitTargetBranchCreateCmd.Flags().String("branch", "", "Branch name or regex pattern (required)")
	gitTargetBranchCreateCmd.Flags().Bool("regex", false, "Treat the branch pattern as a regular expression")

	gitTargetBranchUpdateCmd.Flags().String("branch", "", "New branch name or regex pattern")
	gitTargetBranchUpdateCmd.Flags().Bool("regex", false, "Treat the branch pattern as a regular expression")
}
