package cmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// batchIssueRow is one row of a batch-create file (JSON object or CSV row).
// title and team are required; the rest are optional. It mirrors the flags on
// `issue create`. Labels may be a JSON array; in CSV, a semicolon-separated
// cell (e.g. "Bug;Backend").
type batchIssueRow struct {
	Title       string   `json:"title"`
	Team        string   `json:"team"`
	Description string   `json:"description"`
	Assignee    string   `json:"assignee"`
	State       string   `json:"state"`
	Priority    *int     `json:"priority"`
	Project     string   `json:"project"`
	Labels      []string `json:"labels"`
	Cycle       string   `json:"cycle"`
	Milestone   string   `json:"milestone"`
	Estimate    *int     `json:"estimate"`
	DueDate     string   `json:"dueDate"`
	Parent      string   `json:"parent"`
}

// retryOnRateLimit runs fn, retrying up to 3 times with exponential backoff
// when the error looks like a Linear rate-limit (HTTP 429 / RATELIMITED).
// Other errors are returned immediately - only transient throttling is worth
// retrying for a batch mutation.
func retryOnRateLimit(fn func() error) error {
	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	var err error
	for attempt := 0; attempt <= len(backoff); attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		if !isRateLimitError(err) || attempt == len(backoff) {
			return err
		}
		time.Sleep(backoff[attempt])
	}
	return err
}

func isRateLimitError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "ratelimit") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "429") ||
		strings.Contains(msg, "too many requests")
}

var issueBatchCreateCmd = &cobra.Command{
	Use:   "batch-create",
	Short: "Create multiple issues from a file",
	Long: `Create multiple issues in one transaction from a JSON or CSV file.

The format is detected from the file extension (.json or .csv); override with
--format. Each row needs at least a title and a team.

JSON: an array of objects, e.g.
  [{"title": "Fix login", "team": "ENG", "priority": 1, "labels": ["Bug"]}]

CSV: a header row naming columns, then one issue per row. The label column may
list several labels separated by semicolons (e.g. "Bug;Backend"). Supported
columns: title, team, description, assignee, state, priority, project, labels,
cycle, milestone, estimate, due-date, parent.

All rows are resolved before anything is created; if any row fails to resolve,
nothing is created.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			return errors.New("A file is required (--file)")
		}

		rows, err := parseBatchFile(file, cmd)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return errors.New("No issues found in file")
		}
		if len(rows) > 50 {
			return fmt.Errorf("batch-create accepts at most 50 issues at a time (file has %d)", len(rows))
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		// Resolve every row up front. The cache means repeated teams, labels,
		// assignees, and so on are only looked up once across the whole file.
		inputs := make([]*api.IssueCreateInput, 0, len(rows))
		for i, row := range rows {
			input, err := buildBatchCreateInput(ctx, client, cache, row)
			if err != nil {
				return fmt.Errorf("Row %d: %v", i+1, err)
			}
			inputs = append(inputs, input)
		}

		var resp *api.IssueBatchCreateResponse
		err = retryOnRateLimit(func() error {
			var e error
			resp, e = api.IssueBatchCreate(ctx, client, &api.IssueBatchCreateInput{Issues: inputs})
			return e
		})
		if err != nil {
			return fmt.Errorf("Failed to create issues: %v", err)
		}
		if !resp.IssueBatchCreate.Success {
			return errors.New("Failed to create issues")
		}

		created := resp.IssueBatchCreate.Issues
		if jsonOut {
			output.JSON(created)
			return nil
		}
		for _, issue := range created {
			fmt.Printf("Created %s: %s\n", issue.IssueListFields.Identifier, issue.IssueListFields.Title)
		}
		output.Success(fmt.Sprintf("Created %d issues", len(created)), plaintext, jsonOut)
		return nil
	},
}

var issueBatchUpdateCmd = &cobra.Command{
	Use:   "batch-update <issue-id>...",
	Short: "Apply the same update to multiple issues",
	Long: `Apply one set of changes to multiple issues (up to 50) in one transaction.

Pass the issues as arguments (identifiers like TEAM-123 or UUIDs); the flags
describe the single update applied to all of them.

Name-based --state, --cycle, and --milestone resolve against the first issue's
team/project, so those flags assume the issues share a team (and project).

Examples:
  lincli issue batch-update ENG-1 ENG-2 ENG-3 --state Done
  lincli issue batch-update ENG-1 ENG-2 --assignee me --priority 1`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		if len(args) > 50 {
			return errors.New("batch-update accepts at most 50 issues at a time")
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache := newResolverCache()

		// Resolve each issue reference to a UUID (issueBatchUpdate requires
		// UUIDs). Remember the first issue's team/project as the context for
		// name-based --state/--cycle/--milestone resolution.
		ids := make([]string, 0, len(args))
		var firstDetail *api.IssueDetailFields
		for _, ref := range args {
			if isUUID(ref) {
				ids = append(ids, ref)
				continue
			}
			detail, err := resolveIssueDetail(ctx, client, ref)
			if err != nil {
				return fmt.Errorf("Failed to find %v", err)
			}
			ids = append(ids, detail.Id)
			if firstDetail == nil {
				firstDetail = detail
			}
		}

		input := buildIssueUpdateInput(cmd)

		if cmd.Flags().Changed("assignee") {
			assignee, _ := cmd.Flags().GetString("assignee")
			if strings.EqualFold(assignee, "unassigned") || assignee == "" {
				// Clear the assignee via the null sentinel; a bare nil pointer
				// is stripped by the client and would be a silent no-op.
				nullVal := api.NullSentinel
				input.AssigneeId = &nullVal
			} else {
				userID, err := resolveUser(ctx, client, cache, assignee)
				if err != nil {
					return err
				}
				input.AssigneeId = &userID
			}
		}
		if cmd.Flags().Changed("label") {
			ids, err := resolveIssueLabels(ctx, client, cache, cmd)
			if err != nil {
				return err
			}
			input.LabelIds = ids
		}
		if cmd.Flags().Changed("project") {
			projectName, _ := cmd.Flags().GetString("project")
			if strings.EqualFold(projectName, "none") || projectName == "" {
				nullVal := api.NullSentinel
				input.ProjectId = &nullVal
			} else {
				projectID, err := resolveProject(ctx, client, cache, projectName)
				if err != nil {
					return err
				}
				input.ProjectId = &projectID
			}
		}
		if cmd.Flags().Changed("state") {
			stateName, _ := cmd.Flags().GetString("state")
			if firstDetail == nil {
				return errors.New("Cannot resolve --state by name from a UUID-only batch. Pass a state UUID instead.")
			}
			stateID, err := resolveWorkflowState(ctx, client, cache, firstDetail.Team.Key, stateName)
			if err != nil {
				return err
			}
			input.StateId = &stateID
		}
		if cmd.Flags().Changed("cycle") {
			cycle, _ := cmd.Flags().GetString("cycle")
			teamID := ""
			if !isUUID(cycle) {
				if firstDetail == nil {
					return errors.New("Cannot resolve --cycle by name from a UUID-only batch. Pass a cycle UUID instead.")
				}
				teamID = firstDetail.Team.Id
			}
			cycleID, err := resolveCycle(ctx, client, cache, teamID, cycle)
			if err != nil {
				return err
			}
			input.CycleId = &cycleID
		}
		if cmd.Flags().Changed("milestone") {
			milestone, _ := cmd.Flags().GetString("milestone")
			if isUUID(milestone) {
				input.ProjectMilestoneId = &milestone
			} else {
				if firstDetail == nil || firstDetail.Project == nil {
					return errors.New("Cannot resolve --milestone by name: pass a milestone UUID, or ensure the first issue is in a project.")
				}
				milestoneID, err := resolveMilestone(ctx, client, cache, firstDetail.Project.Id, milestone)
				if err != nil {
					return err
				}
				input.ProjectMilestoneId = &milestoneID
			}
		}
		if cmd.Flags().Changed("parent") {
			parent, _ := cmd.Flags().GetString("parent")
			if strings.EqualFold(parent, "none") || parent == "" {
				nullVal := api.NullSentinel
				input.ParentId = &nullVal
			} else {
				parentID, err := resolveParentIssueID(ctx, client, parent)
				if err != nil {
					return err
				}
				input.ParentId = &parentID
			}
		}

		if !batchUpdateHasChanges(cmd, input) {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		var resp *api.IssueBatchUpdateResponse
		err = retryOnRateLimit(func() error {
			var e error
			resp, e = api.IssueBatchUpdate(ctx, client, ids, &input)
			return e
		})
		if err != nil {
			return fmt.Errorf("Failed to update issues: %v", err)
		}
		if !resp.IssueBatchUpdate.Success {
			return errors.New("Failed to update issues")
		}

		if jsonOut {
			output.JSON(resp.IssueBatchUpdate.Issues)
			return nil
		}
		output.Success(fmt.Sprintf("Updated %d issues", len(resp.IssueBatchUpdate.Issues)), plaintext, jsonOut)
		return nil
	},
}

// batchUpdateHasChanges reports whether the batch-update input carries any
// change (pointer field set, or a flag whose handler may set a null sentinel).
func batchUpdateHasChanges(cmd *cobra.Command, input api.IssueUpdateInput) bool {
	return input.Title != nil ||
		input.Description != nil ||
		input.Priority != nil ||
		input.Estimate != nil ||
		input.AssigneeId != nil ||
		input.StateId != nil ||
		input.LabelIds != nil ||
		input.CycleId != nil ||
		input.ProjectMilestoneId != nil ||
		cmd.Flags().Changed("project") ||
		cmd.Flags().Changed("parent")
}

// buildBatchCreateInput resolves one row into an IssueCreateInput, reusing the
// shared cache. Team and title are required.
func buildBatchCreateInput(ctx context.Context, client graphql.Client, cache *ResolverCache, row batchIssueRow) (*api.IssueCreateInput, error) {
	if strings.TrimSpace(row.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	if strings.TrimSpace(row.Team) == "" {
		return nil, fmt.Errorf("team is required")
	}

	teamID, err := resolveTeam(ctx, client, cache, row.Team)
	if err != nil {
		return nil, err
	}

	title := row.Title
	input := &api.IssueCreateInput{TeamId: teamID, Title: &title}

	if row.Description != "" {
		input.Description = &row.Description
	}
	if row.Priority != nil {
		input.Priority = row.Priority
	}
	if row.Estimate != nil {
		input.Estimate = row.Estimate
	}
	if row.DueDate != "" {
		input.DueDate = &row.DueDate
	}
	if row.Assignee != "" {
		userID, err := resolveUser(ctx, client, cache, row.Assignee)
		if err != nil {
			return nil, err
		}
		input.AssigneeId = &userID
	}
	if row.State != "" {
		stateID, err := resolveWorkflowState(ctx, client, cache, teamID, row.State)
		if err != nil {
			return nil, err
		}
		input.StateId = &stateID
	}
	if len(row.Labels) > 0 {
		ids := make([]string, 0, len(row.Labels))
		for _, label := range row.Labels {
			id, err := resolveLabel(ctx, client, cache, label)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		input.LabelIds = ids
	}
	if row.Cycle != "" {
		cycleID, err := resolveCycle(ctx, client, cache, teamID, row.Cycle)
		if err != nil {
			return nil, err
		}
		input.CycleId = &cycleID
	}
	if row.Parent != "" {
		parentID, err := resolveIssueID(ctx, client, row.Parent)
		if err != nil {
			return nil, fmt.Errorf("parent %w", err)
		}
		input.ParentId = &parentID
	}
	// Project must be resolved before milestone (a milestone belongs to it).
	if row.Project != "" {
		projectID, err := resolveProject(ctx, client, cache, row.Project)
		if err != nil {
			return nil, err
		}
		input.ProjectId = &projectID
	}
	if row.Milestone != "" {
		if input.ProjectId == nil {
			return nil, fmt.Errorf("milestone requires a project")
		}
		milestoneID, err := resolveMilestone(ctx, client, cache, *input.ProjectId, row.Milestone)
		if err != nil {
			return nil, err
		}
		input.ProjectMilestoneId = &milestoneID
	}

	return input, nil
}

// parseBatchFile reads a batch-create file as JSON or CSV. The format is taken
// from --format, or inferred from the file extension when --format is "auto".
func parseBatchFile(path string, cmd *cobra.Command) ([]batchIssueRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	format, _ := cmd.Flags().GetString("format")
	if format == "" || format == "auto" {
		if strings.HasSuffix(strings.ToLower(path), ".csv") {
			format = "csv"
		} else {
			format = "json"
		}
	}

	switch format {
	case "json":
		var rows []batchIssueRow
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return rows, nil
	case "csv":
		return parseBatchCSV(data)
	default:
		return nil, fmt.Errorf("invalid --format '%s' (expected json, csv, or auto)", format)
	}
}

// parseBatchCSV parses CSV bytes into rows using the header line to map
// columns. Unknown columns are ignored; the label column is split on ';'.
func parseBatchCSV(data []byte) ([]batchIssueRow, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV needs a header row and at least one data row")
	}

	headers := records[0]
	rows := make([]batchIssueRow, 0, len(records)-1)
	for rowNum, record := range records[1:] {
		var row batchIssueRow
		for i, header := range headers {
			if i >= len(record) {
				break
			}
			value := strings.TrimSpace(record[i])
			if value == "" {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(header)) {
			case "title":
				row.Title = value
			case "team":
				row.Team = value
			case "description":
				row.Description = value
			case "assignee":
				row.Assignee = value
			case "state":
				row.State = value
			case "priority":
				n, err := strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("row %d: invalid priority %q (want an integer)", rowNum+1, value)
				}
				row.Priority = &n
			case "project":
				row.Project = value
			case "labels", "label":
				parts := strings.Split(value, ";")
				for _, p := range parts {
					if p = strings.TrimSpace(p); p != "" {
						row.Labels = append(row.Labels, p)
					}
				}
			case "cycle":
				row.Cycle = value
			case "milestone":
				row.Milestone = value
			case "estimate":
				n, err := strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("row %d: invalid estimate %q (want an integer)", rowNum+1, value)
				}
				row.Estimate = &n
			case "due-date", "duedate":
				row.DueDate = value
			case "parent":
				row.Parent = value
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func init() {
	issueCmd.AddCommand(issueBatchCreateCmd)
	issueCmd.AddCommand(issueBatchUpdateCmd)

	issueBatchCreateCmd.Flags().StringP("file", "f", "", "Path to a JSON or CSV file of issues (required)")
	issueBatchCreateCmd.Flags().String("format", "auto", "File format: auto (by extension), json, or csv")

	issueBatchUpdateCmd.Flags().String("title", "", "New title for all issues")
	issueBatchUpdateCmd.Flags().StringP("description", "d", "", "New description for all issues")
	issueBatchUpdateCmd.Flags().StringP("assignee", "a", "", "Assignee (email, name, 'me', or 'unassigned')")
	issueBatchUpdateCmd.Flags().StringP("state", "s", "", "State name (resolved against the first issue's team) or UUID")
	issueBatchUpdateCmd.Flags().Int("priority", -1, "Priority (0=None, 1=Urgent, 2=High, 3=Normal, 4=Low)")
	issueBatchUpdateCmd.Flags().StringSlice("label", nil, "Label name or ID (repeatable; replaces labels)")
	issueBatchUpdateCmd.Flags().String("cycle", "", "Cycle number, name (first issue's team), or UUID")
	issueBatchUpdateCmd.Flags().String("milestone", "", "Project milestone name (first issue's project) or UUID")
	issueBatchUpdateCmd.Flags().Int("estimate", 0, "Estimate (story points)")
	issueBatchUpdateCmd.Flags().String("project", "", "Project name or ID (or 'none' to remove)")
	issueBatchUpdateCmd.Flags().String("parent", "", "Parent issue identifier or ID (or 'none' to clear)")
}
