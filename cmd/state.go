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
  lincli state create --team ENG --name "In Review" --type started --color "#F2C94C"
  lincli state update STATE-ID --name "Code Review"
  lincli state archive STATE-ID`,
}

var stateListCmd = &cobra.Command{
	Use:     "list <team>",
	Aliases: []string{"ls"},
	Short:   "List a team's workflow states",
	Long:    `List the workflow states for a team. Accepts a team key or ID.`,
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
		ctx := context.Background()

		// state list reuses GetTeamStates, which embeds the team's states -
		// the same query the workflow-state resolver uses. Team lookup accepts
		// a key or an ID directly.
		resp, err := api.GetTeamStates(ctx, client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list workflow states: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.Team == nil || resp.Team.States == nil || len(resp.Team.States.Nodes) == 0 {
			output.Info(fmt.Sprintf("No workflow states found for team '%s'", args[0]), plaintext, jsonOut)
			return
		}
		nodes := resp.Team.States.Nodes

		if jsonOut {
			output.JSON(nodes)
			return
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
			return
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
	},
}

var stateGetCmd = &cobra.Command{
	Use:     "get <state-id>",
	Aliases: []string{"show"},
	Short:   "Get workflow state details",
	Long:    `Get detailed information about a workflow state.`,
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
		ctx := context.Background()

		resp, err := api.GetWorkflowState(ctx, client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to get workflow state: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.WorkflowState == nil {
			output.Error(fmt.Sprintf("Workflow state not found: %s", args[0]), plaintext, jsonOut)
			os.Exit(1)
		}
		f := resp.WorkflowState.WorkflowStateFields

		if jsonOut {
			output.JSON(resp.WorkflowState)
			return
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
			return
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
	},
}

var stateCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a new workflow state",
	Long:    `Create a new workflow state for a team. --name, --type, and --color are required.`,
	Run: func(cmd *cobra.Command, args []string) {
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

		team, _ := cmd.Flags().GetString("team")
		name, _ := cmd.Flags().GetString("name")
		stateType, _ := cmd.Flags().GetString("type")
		stateColor, _ := cmd.Flags().GetString("color")

		switch {
		case team == "":
			output.Error("Team is required (--team)", plaintext, jsonOut)
			os.Exit(1)
		case name == "":
			output.Error("Name is required (--name)", plaintext, jsonOut)
			os.Exit(1)
		case stateType == "":
			output.Error("Type is required (--type)", plaintext, jsonOut)
			os.Exit(1)
		case stateColor == "":
			output.Error("Color is required (--color)", plaintext, jsonOut)
			os.Exit(1)
		}
		if !isValidStateType(stateType) {
			output.Error(fmt.Sprintf("Invalid --type '%s'. Valid values: backlog, unstarted, started, completed, canceled", stateType), plaintext, jsonOut)
			os.Exit(1)
		}

		teamID, err := resolveTeam(ctx, client, cache, team)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
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
			output.Error(fmt.Sprintf("Failed to create workflow state: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.WorkflowStateCreate.Success {
			output.Error("Failed to create workflow state", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.WorkflowStateCreate.WorkflowState)
		} else {
			output.Success(fmt.Sprintf("Created workflow state %s", resp.WorkflowStateCreate.WorkflowState.WorkflowStateFields.Name), plaintext, jsonOut)
		}
	},
}

var stateUpdateCmd = &cobra.Command{
	Use:   "update <state-id>",
	Short: "Update a workflow state",
	Long:  `Update a workflow state's name, description, or color.`,
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
			output.Error("No updates specified. Use flags to specify what to update.", plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.WorkflowStateUpdate(ctx, client, args[0], &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update workflow state: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.WorkflowStateUpdate.Success {
			output.Error("Failed to update workflow state", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.WorkflowStateUpdate.WorkflowState)
		} else {
			output.Success(fmt.Sprintf("Updated workflow state %s", args[0]), plaintext, jsonOut)
		}
	},
}

var stateArchiveCmd = &cobra.Command{
	Use:   "archive <state-id>",
	Short: "Archive a workflow state",
	Long:  `Archive a workflow state (Linear has no hard delete). Executes immediately with no confirmation prompt.`,
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
		ctx := context.Background()

		resp, err := api.WorkflowStateArchive(ctx, client, args[0])
		if err != nil {
			output.Error(fmt.Sprintf("Failed to archive workflow state: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if !resp.WorkflowStateArchive.Success {
			output.Error("Failed to archive workflow state", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Archived workflow state %s", args[0]), plaintext, jsonOut)
		}
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
	stateCreateCmd.Flags().String("type", "", "State type: backlog, unstarted, started, completed, canceled (required)")
	stateCreateCmd.Flags().String("color", "", "State color as a HEX string, e.g. #F2C94C (required)")
	stateCreateCmd.Flags().StringP("description", "d", "", "State description")
	stateCreateCmd.Flags().Float64("position", 0, "Ordering position within the team's workflow")

	stateUpdateCmd.Flags().String("name", "", "State name")
	stateUpdateCmd.Flags().String("color", "", "State color as a HEX string (e.g. #F2C94C)")
	stateUpdateCmd.Flags().StringP("description", "d", "", "State description")
	stateUpdateCmd.Flags().Float64("position", 0, "Ordering position within the team's workflow")
}
