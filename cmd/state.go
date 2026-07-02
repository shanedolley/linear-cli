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

// workflowStateTypes are the valid WorkflowState.type values, used to validate
// the --type flag on create before hitting the API.
var workflowStateTypes = []string{"backlog", "unstarted", "started", "completed", "canceled"}

func isValidStateType(t string) bool {
	for _, v := range workflowStateTypes {
		if v == t {
			return true
		}
	}
	return false
}

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage Linear workflow states",
	Long: `Manage Linear workflow states: the per-team issue statuses (Backlog,
Todo, In Progress, Done, and so on).

Linear has no hard delete for states - use 'state archive' instead.

Examples:
  lincli state list ENG                    # List a team's workflow states
  lincli state get STATE-ID
  lincli state create --team ENG --name "In Review" --state-type started --color "#F2C94C"
  lincli state update STATE-ID --name "Code Review"
  lincli state archive STATE-ID`,
}

var stateListCmd = &cobra.Command{
	Use:     "list <team>",
	Aliases: []string{"ls"},
	Short:   "List a team's workflow states",
	Long:    `List the workflow states for a team. Accepts a team key or ID.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		// state list reuses GetTeamStates, which embeds the team's states -
		// the same query the workflow-state resolver uses. Team lookup accepts
		// a key or an ID directly.
		resp, err := api.GetTeamStates(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to list workflow states: %v", err)
		}
		if resp.Team == nil || resp.Team.States == nil || len(resp.Team.States.Nodes) == 0 {
			output.Info(fmt.Sprintf("No workflow states found for team '%s'", args[0]), plaintext, jsonOut)
			return nil
		}
		nodes := resp.Team.States.Nodes

		if jsonOut {
			output.JSON(nodes)
			return nil
		}

		if plaintext {
			fmt.Println("# Workflow States")
			for _, s := range nodes {
				fmt.Printf("## %s\n", s.Name)
				fmt.Printf("- **ID**: %s\n", s.Id)
				fmt.Printf("- **Type**: %s\n", s.Type)
				fmt.Printf("- **Color**: %s\n", s.Color)
				fmt.Printf("- **Position**: %.0f\n", s.Position)
				fmt.Println()
			}
			fmt.Printf("\nTotal: %d states\n", len(nodes))
			return nil
		}

		headers := []string{"ID", "Name", "Type", "Color", "Position"}
		rows := make([][]string, len(nodes))
		for i, s := range nodes {
			rows[i] = []string{s.Id, truncateString(s.Name, 30), s.Type, s.Color, fmt.Sprintf("%.0f", s.Position)}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d states\n", color.New(color.FgGreen).Sprint("✓"), len(nodes))
		}
		return nil
	},
}

var stateGetCmd = &cobra.Command{
	Use:     "get <state-id>",
	Aliases: []string{"show"},
	Short:   "Get workflow state details",
	Long:    `Get detailed information about a workflow state.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.GetWorkflowState(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to get workflow state: %v", err)
		}
		if resp.WorkflowState == nil {
			return fmt.Errorf("Workflow state not found: %s", args[0])
		}
		f := resp.WorkflowState.WorkflowStateFields

		if jsonOut {
			output.JSON(resp.WorkflowState)
			return nil
		}

		if plaintext {
			fmt.Printf("# %s\n\n", f.Name)
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Type**: %s\n", f.Type)
			fmt.Printf("- **Color**: %s\n", f.Color)
			fmt.Printf("- **Position**: %.0f\n", f.Position)
			fmt.Printf("- **Team**: %s\n", f.Team.Key)
			if f.Description != nil && *f.Description != "" {
				fmt.Printf("- **Description**: %s\n", *f.Description)
			}
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("State:"), f.Name)
		fmt.Printf("  ID:    %s\n", f.Id)
		fmt.Printf("  Type:  %s\n", f.Type)
		fmt.Printf("  Color: %s\n", f.Color)
		fmt.Printf("  Position: %.0f\n", f.Position)
		fmt.Printf("  Team:  %s\n", f.Team.Key)
		if f.Description != nil && *f.Description != "" {
			fmt.Printf("  Description: %s\n", *f.Description)
		}
		return nil
	},
}

var stateCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a new workflow state",
	Long:    `Create a new workflow state for a team. --name, --state-type, and --color are required.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		team, _ := cmd.Flags().GetString("team")
		name, _ := cmd.Flags().GetString("name")
		stateType := preferFlag(cmd, "state-type", "type")
		stateColor, _ := cmd.Flags().GetString("color")

		switch {
		case team == "":
			return errors.New("Team is required (--team)")
		case name == "":
			return errors.New("Name is required (--name)")
		case stateType == "":
			return errors.New("Type is required (--state-type)")
		case stateColor == "":
			return errors.New("Color is required (--color)")
		}
		if !isValidStateType(stateType) {
			return fmt.Errorf("Invalid --state-type '%s'. Valid values: backlog, unstarted, started, completed, canceled", stateType)
		}

		teamID, err := resolveTeam(ctx, client, cache, team)
		if err != nil {
			return err
		}

		input := api.WorkflowStateCreateInput{
			TeamId: teamID,
			Name:   name,
			Type:   stateType,
			Color:  stateColor,
		}
		if description, _ := cmd.Flags().GetString("description"); description != "" {
			input.Description = &description
		}
		if cmd.Flags().Changed("position") {
			position, _ := cmd.Flags().GetFloat64("position")
			input.Position = &position
		}

		resp, err := api.WorkflowStateCreate(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to create workflow state: %v", err)
		}
		if !resp.WorkflowStateCreate.Success {
			return errors.New("Failed to create workflow state")
		}

		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		if jsonOut {
			output.JSON(resp.WorkflowStateCreate.WorkflowState)
		} else {
			output.Success(fmt.Sprintf("Created workflow state %s", resp.WorkflowStateCreate.WorkflowState.WorkflowStateFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var stateUpdateCmd = &cobra.Command{
	Use:   "update <state-id>",
	Short: "Update a workflow state",
	Long:  `Update a workflow state's name, description, or color.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		input := api.WorkflowStateUpdateInput{}
		hasUpdates := false

		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
			hasUpdates = true
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
			hasUpdates = true
		}
		if cmd.Flags().Changed("color") {
			c, _ := cmd.Flags().GetString("color")
			input.Color = &c
			hasUpdates = true
		}
		if cmd.Flags().Changed("position") {
			position, _ := cmd.Flags().GetFloat64("position")
			input.Position = &position
			hasUpdates = true
		}

		if !hasUpdates {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.WorkflowStateUpdate(ctx, client, args[0], &input)
		if err != nil {
			return fmt.Errorf("Failed to update workflow state: %v", err)
		}
		if !resp.WorkflowStateUpdate.Success {
			return errors.New("Failed to update workflow state")
		}

		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		if jsonOut {
			output.JSON(resp.WorkflowStateUpdate.WorkflowState)
		} else {
			output.Success(fmt.Sprintf("Updated workflow state %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var stateArchiveCmd = &cobra.Command{
	Use:   "archive <state-id>",
	Short: "Archive a workflow state",
	Long:  `Archive a workflow state (Linear has no hard delete). Executes immediately with no confirmation prompt.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.WorkflowStateArchive(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to archive workflow state: %v", err)
		}
		if !resp.WorkflowStateArchive.Success {
			return errors.New("Failed to archive workflow state")
		}

		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Archived workflow state %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stateCmd)
	stateCmd.AddCommand(stateListCmd)
	stateCmd.AddCommand(stateGetCmd)
	stateCmd.AddCommand(stateCreateCmd)
	stateCmd.AddCommand(stateUpdateCmd)
	stateCmd.AddCommand(stateArchiveCmd)

	stateCreateCmd.Flags().StringP("team", "t", "", "Team key (required)")
	stateCreateCmd.Flags().String("name", "", "State name (required)")
	stateCreateCmd.Flags().String("state-type", "", "State type: backlog, unstarted, started, completed, canceled (required)")
	stateCreateCmd.Flags().String("type", "", "Deprecated alias for --state-type")
	_ = stateCreateCmd.Flags().MarkHidden("type")
	stateCreateCmd.Flags().String("color", "", "State color as a HEX string, e.g. #F2C94C (required)")
	stateCreateCmd.Flags().StringP("description", "d", "", "State description")
	stateCreateCmd.Flags().Float64("position", 0, "Ordering position within the team's workflow")

	stateUpdateCmd.Flags().String("name", "", "State name")
	stateUpdateCmd.Flags().String("color", "", "State color as a HEX string (e.g. #F2C94C)")
	stateUpdateCmd.Flags().StringP("description", "d", "", "State description")
	stateUpdateCmd.Flags().Float64("position", 0, "Ordering position within the team's workflow")
}
