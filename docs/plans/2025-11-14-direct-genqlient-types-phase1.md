# Direct genqlient Types - Phase 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate read operations (`list`, `get`, `search`) to use genqlient-generated types directly, eliminating the adapter layer for these commands and achieving full compile-time type safety.

**Architecture:** Commands call generated functions directly (e.g., `GetIssue`, `ListIssues`), build typed filters using generated structs (`IssueFilter`, `NullableUserFilter`), and work with generated response types (`GetIssueResponse`, `IssueDetailFields`). Output layer handles generated types for table/plaintext rendering.

**Tech Stack:** Go 1.24, genqlient v0.8.1, Cobra CLI framework, Linear GraphQL API

---

## Task 1: Migrate `issue get` Command

**Files:**
- Modify: `cmd/issue.go:260-450`

**Step 1: Add import for generated types**

At the top of `cmd/issue.go`, after the existing imports, verify the api package is imported (it should already be):

```go
import (
    // ... existing imports ...
    "github.com/shanedolley/lincli/pkg/api"
)
```

No changes needed if already present.

**Step 2: Replace GetIssue call with generated function**

Locate the `issueGetCmd` Run function around line 276. Replace this line:

```go
issue, err := client.GetIssue(context.Background(), args[0])
```

With:

```go
resp, err := api.GetIssue(context.Background(), client, args[0])
if err != nil {
    output.Error(fmt.Sprintf("Failed to fetch issue: %v", err), plaintext, jsonOut)
    os.Exit(1)
}
issue := resp.Issue
```

**Step 3: Update field access for IssueDetailFields**

The generated type uses `IssueDetailFields` which has slightly different field names. Update field access patterns:

Find line ~296 and update:
```go
// Before
fmt.Printf("- **Number**: %d\n", issue.Number)

// After
fmt.Printf("- **Number**: %.0f\n", issue.IssueDetailFields.Number)
```

Find all remaining field accesses and update to use `issue.IssueDetailFields.FieldName`:
- `issue.Identifier` → `issue.IssueDetailFields.Identifier`
- `issue.Title` → `issue.IssueDetailFields.Title`
- `issue.Description` → `issue.IssueDetailFields.Description` (check for nil - it's `*string`)
- `issue.State` → `issue.IssueDetailFields.State`
- `issue.Assignee` → `issue.IssueDetailFields.Assignee`
- Continue for all fields in the plaintext and table rendering sections

**Step 4: Handle Description nil check**

Around line 290, update description handling:

```go
// Before
if issue.Description != "" {
    fmt.Printf("## Description\n%s\n\n", issue.Description)
}

// After
if issue.IssueDetailFields.Description != nil && *issue.IssueDetailFields.Description != "" {
    fmt.Printf("## Description\n%s\n\n", *issue.IssueDetailFields.Description)
}
```

**Step 5: Update JSON output**

Around line 283, update JSON output to use the generated structure:

```go
// Before
if jsonOut {
    output.JSON(issue)
    return
}

// After
if jsonOut {
    output.JSON(issue.IssueDetailFields)
    return
}
```

**Step 6: Test the command**

Build and test:

```bash
make build
./lincli issue get SD-37
./lincli issue get SD-37 --json
./lincli issue get SD-37 --plaintext
```

Expected: All three outputs work correctly, matching previous behavior.

**Step 7: Commit**

```bash
git add cmd/issue.go
git commit -m "feat: migrate issue get to use generated types directly

Replace adapter call with direct GetIssue generated function.
Update field access to use IssueDetailFields structure.
Add nil checks for nullable fields.

Part of Phase 1: Read operations migration."
```

---

## Task 2: Migrate `issue list` Command

**Files:**
- Modify: `cmd/issue.go:30-85` (issueListCmd)
- Modify: `cmd/issue.go:704-755` (buildIssueFilter)
- Add: `cmd/issue.go` (helper functions for filter building)

**Step 1: Add filter helper functions**

At the end of `cmd/issue.go` (after line 779), add these helper functions:

```go
// Filter helper functions for type-safe filter building
func stringEq(val string) *api.StringComparator {
	return &api.StringComparator{Eq: &val}
}

func stringIn(vals []string) *api.StringComparator {
	return &api.StringComparator{In: vals}
}

func stringNin(vals []string) *api.StringComparator {
	return &api.StringComparator{Nin: vals}
}

func boolEq(val bool) *api.BooleanComparator {
	return &api.BooleanComparator{Eq: &val}
}

func intEq(val int) *api.IntComparator {
	return &api.IntComparator{Eq: &val}
}

func dateGte(val string) *api.DateComparator {
	return &api.DateComparator{Gte: &val}
}
```

**Step 2: Create typed buildIssueFilterTyped function**

After the helper functions, add:

```go
// buildIssueFilterTyped builds a typed IssueFilter from command flags
func buildIssueFilterTyped(cmd *cobra.Command) api.IssueFilter {
	filter := api.IssueFilter{}

	// Assignee filter
	if assignee, _ := cmd.Flags().GetString("assignee"); assignee != "" {
		if assignee == "me" {
			filter.Assignee = &api.NullableUserFilter{
				IsMe: boolEq(true),
			}
		} else {
			filter.Assignee = &api.NullableUserFilter{
				Email: stringEq(assignee),
			}
		}
	}

	// State filter
	state, _ := cmd.Flags().GetString("state")
	if state != "" {
		filter.State = &api.WorkflowStateFilter{
			Name: stringEq(state),
		}
	} else {
		// Exclude completed/canceled unless explicitly included
		includeCompleted, _ := cmd.Flags().GetBool("include-completed")
		if !includeCompleted {
			filter.State = &api.WorkflowStateFilter{
				Type: stringNin([]string{"completed", "canceled"}),
			}
		}
	}

	// Team filter
	if team, _ := cmd.Flags().GetString("team"); team != "" {
		filter.Team = &api.NullableTeamFilter{
			Key: stringEq(team),
		}
	}

	// Priority filter
	if priority, _ := cmd.Flags().GetInt("priority"); priority != -1 {
		filter.Priority = &api.NullableIntComparator{
			Eq: &priority,
		}
	}

	// Time filter
	newerThan, _ := cmd.Flags().GetString("newer-than")
	createdAt, err := utils.ParseTimeExpression(newerThan)
	if err != nil {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		output.Error(fmt.Sprintf("Invalid newer-than value: %v", err), plaintext, jsonOut)
		os.Exit(1)
	}
	if createdAt != "" {
		filter.CreatedAt = &api.DateComparator{
			Gte: &createdAt,
		}
	}

	return filter
}
```

**Step 3: Update issueListCmd to use generated function**

In the `issueListCmd` Run function (around line 55-77), replace the API call:

```go
// Before
filter := buildIssueFilter(cmd)
issues, err := client.GetIssues(context.Background(), filter, limit, "", orderBy)

// After
filterTyped := buildIssueFilterTyped(cmd)

// Convert orderBy string to PaginationOrderBy enum
var orderByEnum *api.PaginationOrderBy
if orderBy != "" {
	if orderBy == "createdAt" {
		val := api.PaginationOrderByCreatedAt
		orderByEnum = &val
	} else if orderBy == "updatedAt" {
		val := api.PaginationOrderByUpdatedAt
		orderByEnum = &val
	}
}

// Convert limit to pointer
var limitPtr *int
if limit > 0 {
	limitPtr = &limit
}

resp, err := api.ListIssues(context.Background(), client, filterTyped, limitPtr, nil, orderByEnum)
if err != nil {
	output.Error(fmt.Sprintf("Failed to fetch issues: %v", err), plaintext, jsonOut)
	os.Exit(1)
}
```

**Step 4: Update renderIssueCollection to handle generated types**

Replace the call to `renderIssueCollection` (around line 83) with direct rendering:

```go
// Check if empty
if len(resp.Issues.Nodes) == 0 {
	output.Info("No issues found", plaintext, jsonOut)
	return
}

// JSON output
if jsonOut {
	output.JSON(resp.Issues.Nodes)
	return
}

// Plaintext output
if plaintext {
	fmt.Println("# Issues")
	for _, node := range resp.Issues.Nodes {
		f := node.IssueListFields
		fmt.Printf("## %s\n", f.Title)
		fmt.Printf("- **ID**: %s\n", f.Identifier)
		if f.State != nil {
			fmt.Printf("- **State**: %s\n", f.State.Name)
		}
		if f.Assignee != nil {
			fmt.Printf("- **Assignee**: %s\n", f.Assignee.Name)
		} else {
			fmt.Printf("- **Assignee**: Unassigned\n")
		}
		if f.Team != nil {
			fmt.Printf("- **Team**: %s\n", f.Team.Key)
		}
		fmt.Printf("- **Created**: %s\n", f.CreatedAt.Format("2006-01-02"))
		fmt.Printf("- **URL**: %s\n", f.Url)
		if f.Description != nil && *f.Description != "" {
			fmt.Printf("- **Description**: %s\n", *f.Description)
		}
		fmt.Println()
	}
	fmt.Printf("\nTotal: %d issues\n", len(resp.Issues.Nodes))
	return
}

// Table output
headers := []string{"Title", "State", "Assignee", "Team", "Created", "URL"}
rows := make([][]string, len(resp.Issues.Nodes))

for i, node := range resp.Issues.Nodes {
	f := node.IssueListFields

	assignee := "Unassigned"
	if f.Assignee != nil {
		assignee = f.Assignee.Name
	}

	team := ""
	if f.Team != nil {
		team = f.Team.Key
	}

	state := ""
	if f.State != nil {
		state = f.State.Name
	}

	rows[i] = []string{
		truncateString(f.Title, 50),
		state,
		assignee,
		team,
		f.CreatedAt.Format("2006-01-02"),
		f.Url,
	}
}

output.Table(headers, rows)
fmt.Printf("\nTotal: %d issues\n", len(resp.Issues.Nodes))
```

**Step 5: Test the command**

```bash
make build
./lincli issue list --limit 5
./lincli issue list --limit 5 --json
./lincli issue list --limit 5 --plaintext
./lincli issue list --assignee me --limit 3
./lincli issue list --team SD --limit 3
./lincli issue list --state "In Progress" --limit 3
```

Expected: All outputs work correctly with proper filtering.

**Step 6: Commit**

```bash
git add cmd/issue.go
git commit -m "feat: migrate issue list to use generated types directly

Add typed filter building with helper functions.
Replace adapter call with direct ListIssues function.
Update rendering to work with IssueListFields structure.

Includes:
- Filter helpers: stringEq, boolEq, dateGte, etc.
- buildIssueFilterTyped function for type-safe filters
- Direct ListIssues call with typed parameters
- Updated rendering for all output modes

Part of Phase 1: Read operations migration."
```

---

## Task 3: Migrate `issue search` Command

**Files:**
- Modify: `cmd/issue.go:180-260` (issueSearchCmd)

**Step 1: Update issueSearchCmd to use generated function**

In the `issueSearchCmd` Run function (around line 248), replace the API call:

```go
// Before
filter := buildIssueFilter(cmd)
issues, err := client.IssueSearch(context.Background(), query, filter, limit, "", orderBy, includeArchived)

// After
filterTyped := buildIssueFilterTyped(cmd)

// Convert parameters
var limitPtr *int
if limit > 0 {
	limitPtr = &limit
}

var orderByEnum *api.PaginationOrderBy
if orderBy != "" {
	if orderBy == "createdAt" {
		val := api.PaginationOrderByCreatedAt
		orderByEnum = &val
	} else if orderBy == "updatedAt" {
		val := api.PaginationOrderByUpdatedAt
		orderByEnum = &val
	}
}

includeArchivedPtr := &includeArchived

resp, err := api.SearchIssues(context.Background(), client, query, filterTyped, limitPtr, nil, orderByEnum, includeArchivedPtr)
if err != nil {
	output.Error(fmt.Sprintf("Failed to search issues: %v", err), plaintext, jsonOut)
	os.Exit(1)
}
```

**Step 2: Update rendering (copy from issue list)**

Replace the `renderIssueCollection` call with the same rendering code from Task 2, Step 4. Change `resp.Issues.Nodes` to `resp.IssueSearch.Nodes` and adjust the summary message to "search results" instead of "issues".

**Step 3: Test the command**

```bash
make build
./lincli issue search "bug"
./lincli issue search "bug" --json
./lincli issue search "bug" --plaintext
./lincli issue search "auth" --team SD --limit 3
```

Expected: Search works correctly with proper result rendering.

**Step 4: Commit**

```bash
git add cmd/issue.go
git commit -m "feat: migrate issue search to use generated types directly

Replace adapter call with direct SearchIssues function.
Reuse typed filter building from issue list migration.
Update rendering to work with generated search response.

Part of Phase 1: Read operations migration."
```

---

## Task 4: Migrate Project Commands

**Files:**
- Modify: `cmd/project.go`

**Step 1: Add filter helpers for projects**

At the end of `cmd/project.go`, add:

```go
// Filter helper for projects
func buildProjectFilterTyped(cmd *cobra.Command) api.ProjectFilter {
	filter := api.ProjectFilter{}

	if state, _ := cmd.Flags().GetString("state"); state != "" {
		filter.State = &api.StringComparator{Eq: &state}
	}

	newerThan, _ := cmd.Flags().GetString("newer-than")
	createdAt, err := utils.ParseTimeExpression(newerThan)
	if err != nil {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")
		output.Error(fmt.Sprintf("Invalid newer-than value: %v", err), plaintext, jsonOut)
		os.Exit(1)
	}
	if createdAt != "" {
		filter.CreatedAt = &api.DateComparator{Gte: &createdAt}
	}

	return filter
}
```

**Step 2: Update project list command**

Find `projectListCmd` Run function and replace:

```go
// Before
filter := buildProjectFilter(cmd)
projects, err := client.GetProjects(context.Background(), filter, limit, "")

// After
filterTyped := buildProjectFilterTyped(cmd)

var limitPtr *int
if limit > 0 {
	limitPtr = &limit
}

resp, err := api.ListProjects(context.Background(), client, filterTyped, limitPtr, nil, nil)
```

Update rendering to use `resp.Projects.Nodes` and `node.ProjectListFields`.

**Step 3: Update project get command**

Find `projectGetCmd` Run function and replace:

```go
// Before
project, err := client.GetProject(context.Background(), args[0])

// After
resp, err := api.GetProject(context.Background(), client, args[0])
if err != nil {
	output.Error(fmt.Sprintf("Failed to fetch project: %v", err), plaintext, jsonOut)
	os.Exit(1)
}
project := resp.Project
```

Update field access to use `project.ProjectDetailFields`.

**Step 4: Test projects**

```bash
make build
./lincli project list --limit 5
./lincli project get <PROJECT-ID>
```

**Step 5: Commit**

```bash
git add cmd/project.go
git commit -m "feat: migrate project commands to use generated types

Migrate project list and project get to use generated functions.
Add buildProjectFilterTyped for type-safe filtering.
Update rendering for ProjectListFields and ProjectDetailFields.

Part of Phase 1: Read operations migration."
```

---

## Task 5: Migrate Team Commands

**Files:**
- Modify: `cmd/team.go`

**Step 1: Update team list command**

Find `teamListCmd` Run function and replace:

```go
// Before
teams, err := client.GetTeams(context.Background(), limit, "")

// After
var limitPtr *int
if limit > 0 {
	limitPtr = &limit
}

resp, err := api.ListTeams(context.Background(), client, limitPtr, nil, nil)
```

Update rendering to use `resp.Teams.Nodes` and `node.TeamListFields`.

**Step 2: Update team get command**

Find `teamGetCmd` Run function and replace:

```go
// Before
team, err := client.GetTeam(context.Background(), teamKey)

// After
resp, err := api.GetTeam(context.Background(), client, teamKey)
if err != nil {
	output.Error(fmt.Sprintf("Failed to fetch team: %v", err), plaintext, jsonOut)
	os.Exit(1)
}
team := resp.Team
```

Update field access to use `team.TeamDetailFields`.

**Step 3: Update team members command**

Find `teamMembersCmd` Run function and replace:

```go
// Before
members, err := client.GetTeamMembers(context.Background(), teamKey)

// After
resp, err := api.GetTeamMembers(context.Background(), client, teamKey)
if err != nil {
	output.Error(fmt.Sprintf("Failed to get team members: %v", err), plaintext, jsonOut)
	os.Exit(1)
}
members := resp.Team.Members.Nodes
```

Update rendering to iterate over `members` array directly and access fields like `member.Name`, `member.Email`, etc.

**Step 4: Test teams**

```bash
make build
./lincli team list
./lincli team get SD
./lincli team members SD
```

**Step 5: Commit**

```bash
git add cmd/team.go
git commit -m "feat: migrate team commands to use generated types

Migrate team list, get, and members to use generated functions.
Update rendering for TeamListFields and TeamDetailFields.
Direct access to team members from generated response.

Part of Phase 1: Read operations migration."
```

---

## Task 6: Migrate User Commands

**Files:**
- Modify: `cmd/user.go`

**Step 1: Update user list command**

Find `userListCmd` Run function and replace:

```go
// Before
users, err := client.GetUsers(context.Background(), limit, "")

// After
var limitPtr *int
if limit > 0 {
	limitPtr = &limit
}

resp, err := api.ListUsers(context.Background(), client, limitPtr, nil, nil)
```

Update rendering to use `resp.Users.Nodes` and access fields directly.

**Step 2: Update whoami command**

Find `whoamiCmd` Run function and replace:

```go
// Before
user, err := client.GetViewer(context.Background())

// After
resp, err := api.GetViewer(context.Background(), client)
if err != nil {
	output.Error(fmt.Sprintf("Failed to fetch viewer: %v", err), plaintext, jsonOut)
	os.Exit(1)
}
user := resp.Viewer
```

Update field access to use `user.UserFields`.

**Step 3: Test users**

```bash
make build
./lincli user list --limit 5
./lincli user whoami
```

**Step 4: Commit**

```bash
git add cmd/user.go
git commit -m "feat: migrate user commands to use generated types

Migrate user list and whoami to use generated functions.
Update rendering for generated user response types.

Part of Phase 1: Read operations migration."
```

---

## Task 7: Migrate Comment Command

**Files:**
- Modify: `cmd/comment.go`

**Step 1: Update comment list command**

Find `commentListCmd` Run function and replace:

```go
// Before
comments, err := client.GetComments(context.Background(), issueID, limit)

// After
var limitPtr *int
if limit > 0 {
	limitPtr = &limit
}

resp, err := api.GetComments(context.Background(), client, issueID, limitPtr)
```

Update rendering to use `resp.Issue.Comments.Nodes` and access comment fields directly.

**Step 2: Test comments**

```bash
make build
./lincli comment list SD-37
./lincli comment list SD-37 --json
```

**Step 3: Commit**

```bash
git add cmd/comment.go
git commit -m "feat: migrate comment list to use generated types

Migrate comment list to use generated GetComments function.
Update rendering for generated comment response types.

Part of Phase 1: Read operations migration."
```

---

## Task 8: Run Full Test Suite

**Files:**
- Test: All commands via smoke tests

**Step 1: Build binary**

```bash
make build
```

Expected: Build succeeds with no errors.

**Step 2: Run smoke tests**

```bash
./smoke_test.sh
```

Expected: All 39 tests pass.

**Step 3: If any tests fail**

- Review failure output
- Check which command failed
- Manually test that command: `./lincli <command> --json`
- Fix any issues with field access or nil handling
- Rebuild and retest

**Step 4: Verify no regressions**

Spot check a few commands manually:
```bash
./lincli issue get SD-37
./lincli issue list --limit 3
./lincli project list --limit 3
./lincli team list
```

Expected: All produce expected output in all formats (table, plaintext, JSON).

---

## Task 9: Mark Old Functions as Deprecated

**Files:**
- Modify: `pkg/api/adapter.go` (add deprecation comments)
- Modify: `cmd/issue.go` (remove old buildIssueFilter function)

**Step 1: Remove old buildIssueFilter function**

In `cmd/issue.go`, delete the old `buildIssueFilter` function (around line 704) since it's now replaced by `buildIssueFilterTyped`.

**Step 2: Add deprecation comments to adapter.go**

Add comments to the top of `pkg/api/adapter.go`:

```go
// This file provides adapter functions that wrap genqlient-generated code
// while maintaining backward compatibility with write operations.
//
// READ OPERATIONS DEPRECATED: issue get/list/search, project get/list,
// team get/list/members, user list/whoami, comment list now use generated
// types directly. These adapters remain only for write operations (create, update).
//
// Phase 2 will migrate write operations, after which this file can be deleted.
```

**Step 3: Verify build still works**

```bash
make build
```

Expected: Build succeeds, any references to removed functions cause compile errors (there should be none).

**Step 4: Commit**

```bash
git add pkg/api/adapter.go cmd/issue.go
git commit -m "chore: mark read operation adapters as deprecated

Remove old buildIssueFilter function (replaced by buildIssueFilterTyped).
Add deprecation comment to adapter.go noting read operations now use
generated types directly.

Phase 1 complete: All read operations migrated.
Phase 2 next: Migrate write operations (create, update, delete)."
```

---

## Task 10: Update Documentation

**Files:**
- Modify: `CLAUDE.md` (update architecture section)

**Step 1: Update CLAUDE.md architecture section**

Find the "API Client" section (around line 88) and update:

```markdown
#### API Client
- **Location**: `pkg/api/client.go`
- **Pattern**: Implements `graphql.Client` interface for genqlient integration
- **Core methods**:
  - `MakeRequest()`: Required by genqlient for generated code
  - `Execute()`: Legacy method for backward compatibility (deprecated)
- **Usage**: Commands call genqlient-generated functions directly for read operations
- **Adapter layer**: `pkg/api/adapter.go` maintained only for write operations (create, update, delete)
- **Code generation**: Operations defined in `pkg/api/operations/*.graphql` are compiled to type-safe Go code via genqlient

**Migration Status:**
- ✅ Phase 1 Complete: Read operations (list, get, search) use generated types directly
- ⏳ Phase 2 Planned: Write operations (create, update, delete) will be migrated
- ⏳ Phase 3 Planned: Remove adapter.go and complete migration
```

**Step 2: Commit documentation**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for Phase 1 completion

Document that read operations now use generated types directly.
Note adapter layer only remains for write operations.
Outline Phase 2 and Phase 3 plans.

Phase 1 complete: 8 read commands migrated to direct genqlient usage."
```

---

## Task 11: Create Completion Report

**Files:**
- Create: `docs/verification/2025-11-14-direct-types-phase1.md`

**Step 1: Create verification report**

```markdown
# Direct genqlient Types - Phase 1 Verification Report

**Date:** 2025-11-14
**Branch:** direct-genqlient-types
**Status:** Complete

## Phase 1: Read Operations

### Commands Migrated

**Issues (3 commands):**
- ✅ `issue get` - Direct GetIssue function
- ✅ `issue list` - Typed filters with ListIssues
- ✅ `issue search` - Typed filters with SearchIssues

**Projects (2 commands):**
- ✅ `project list` - Direct ListProjects
- ✅ `project get` - Direct GetProject

**Teams (3 commands):**
- ✅ `team list` - Direct ListTeams
- ✅ `team get` - Direct GetTeam
- ✅ `team members` - Direct GetTeamMembers

**Users (2 commands):**
- ✅ `user list` - Direct ListUsers
- ✅ `user whoami` - Direct GetViewer

**Comments (1 command):**
- ✅ `comment list` - Direct GetComments

### Changes Summary

**Files Modified:**
- `cmd/issue.go` - Migrated 3 commands, added typed filter helpers
- `cmd/project.go` - Migrated 2 commands
- `cmd/team.go` - Migrated 3 commands
- `cmd/user.go` - Migrated 2 commands
- `cmd/comment.go` - Migrated 1 command
- `pkg/api/adapter.go` - Added deprecation comments
- `CLAUDE.md` - Updated architecture documentation

**New Code:**
- Typed filter building functions
- Helper functions for common filter patterns
- Direct rendering for generated response types

**Deleted Code:**
- Old map-based buildIssueFilter function

### Benefits Achieved

**Type Safety:**
- All filters compile-time checked
- Field access validated by compiler
- IDE autocomplete for all filter fields

**Code Quality:**
- ~200 lines of adapter code no longer used
- Cleaner command code with typed structures
- Explicit nil handling for nullable fields

**Maintainability:**
- Schema changes caught at compile time
- No manual filter map construction
- Single source of truth (generated types)

### Test Results

**Build:** ✅ Success
**Smoke Tests:** ✅ All 39 tests passing
**Manual Verification:** ✅ All migrated commands tested

### Next Steps

**Phase 2:** Migrate write operations (create, update, delete)
**Phase 3:** Delete adapter.go, complete migration

## Verification

Run full test suite:
```bash
make build
./smoke_test.sh
```

All tests passing confirms Phase 1 complete.
```

**Step 2: Commit verification report**

```bash
git add docs/verification/2025-11-14-direct-types-phase1.md
git commit -m "docs: add Phase 1 verification report

Document completion of read operations migration:
- All 11 read commands migrated
- All 39 smoke tests passing
- Type safety achieved for read operations
- Adapter layer usage reduced

Phase 1 complete. Ready for Phase 2 planning."
```

---

## Success Criteria

Phase 1 completes when:

- ✅ All 11 read commands migrated (issue get/list/search, project list/get, team list/get/members, user list/whoami, comment list)
- ✅ All commands use generated functions directly
- ✅ Typed filter building implemented
- ✅ All 39 smoke tests passing
- ✅ Build succeeds with no errors
- ✅ No regressions in output (table, plaintext, JSON)
- ✅ Documentation updated
- ✅ Verification report created

## Execution Notes

**Recommended approach:** Use superpowers:subagent-driven-development for task-by-task execution with code reviews between tasks.

**Testing strategy:** Run `make build` after each task, run smoke tests after every 2-3 tasks to catch regressions early.

**If compilation errors occur:** Check field name mappings (generated types may have different casing), verify nil checks for nullable fields, ensure proper pointer conversions.

**Time estimate:** ~2-3 hours for all 11 tasks (Task 1 establishes pattern, remaining tasks apply it).
