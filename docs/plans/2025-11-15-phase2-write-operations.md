# Phase 2: Direct genqlient Types - Write Operations

**Date:** 2025-11-15
**Status:** Design Approved
**Author:** Architecture Review
**Prerequisites:** Phase 1 Complete (all read operations migrated)

## Problem Statement

Phase 1 successfully migrated all read operations to use genqlient-generated types directly. Write operations still use the adapter layer with `map[string]interface{}` inputs, missing the compile-time type safety benefits that Phase 1 achieved for read operations.

**Current flow:**
```
Commands → map inputs → Adapters → Generated types → GraphQL API
```

**Desired flow:**
```
Commands → Typed inputs → Generated types → GraphQL API
```

## Goals

1. **Type safety for inputs** - Replace `map[string]interface{}` with typed structs (`IssueCreateInput`, `IssueUpdateInput`, `CommentCreateInput`)
2. **Direct function calls** - Call generated functions (`api.CreateIssue()`, `api.UpdateIssue()`) instead of adapters
3. **Preserve functionality** - All write operations work identically, no regressions
4. **Incremental migration** - One command at a time with manual testing between

## Non-Goals

- Changing command-line interface (flags, arguments, help text)
- Modifying output formats (table, plaintext, JSON)
- Adding new features or capabilities
- Creating automated tests for write operations (manual testing only)

## Scope

### Commands to Migrate: 4

**Write Operations:**
1. `issue assign` - Assigns issue to current user (simplest)
2. `comment create` - Adds comment to issue (simple)
3. `issue create` - Creates new issue with multiple fields (moderate)
4. `issue update` - Updates existing issue (most complex)

### Migration Order Rationale

**Simplest to most complex** allows us to:
- Prove the basic pattern quickly with `issue assign`
- Add simple user input handling with `comment create`
- Test multiple optional fields with `issue create`
- Tackle state changes and complexity with `issue update`

Each command builds on patterns established by the previous one.

## Technical Approach

### Input Building Patterns

#### Pattern 1: Inline (Simple Commands)

For commands with 1-3 fields, build structs inline in the command handler.

**Example: `issue assign`**
```go
// Get current user
resp, err := api.GetViewer(ctx, client)
if err != nil {
    output.Error(fmt.Sprintf("Failed to get current user: %v", err), plaintext, jsonOut)
    os.Exit(1)
}
viewerID := resp.Viewer.UserDetailFields.Id

// Build input inline
input := api.IssueUpdateInput{
    AssigneeId: &viewerID,
}

// Call generated function
updateResp, err := api.UpdateIssue(ctx, client, issueID, input)
if err != nil {
    output.Error(fmt.Sprintf("Failed to assign issue: %v", err), plaintext, jsonOut)
    os.Exit(1)
}

issue := updateResp.IssueUpdate.Issue
```

**Example: `comment create`**
```go
body, _ := cmd.Flags().GetString("body")
bodyPtr := body  // Copy for pointer

input := api.CommentCreateInput{
    Body:    &bodyPtr,
    IssueId: issueID,
}

resp, err := api.CreateComment(ctx, client, input)
if err != nil {
    output.Error(fmt.Sprintf("Failed to create comment: %v", err), plaintext, jsonOut)
    os.Exit(1)
}

comment := resp.CommentCreate.Comment
```

#### Pattern 2: Builder Functions (Complex Commands)

For commands with 5+ optional fields, create builder functions to construct inputs from flags.

**Example: `buildIssueCreateInput()`**
```go
func buildIssueCreateInput(cmd *cobra.Command, teamID string) api.IssueCreateInput {
    input := api.IssueCreateInput{
        TeamId: teamID,  // Required field (no pointer)
    }

    // Optional string fields
    if title, _ := cmd.Flags().GetString("title"); title != "" {
        input.Title = &title
    }
    if description, _ := cmd.Flags().GetString("description"); description != "" {
        input.Description = &description
    }

    // Optional numeric fields (int → float64 conversion)
    if priority, _ := cmd.Flags().GetInt("priority"); priority >= 0 && priority <= 4 {
        p := float64(priority)
        input.Priority = &p
    }

    return input
}

// Usage in command handler
input := buildIssueCreateInput(cmd, teamID)
if assignToMe {
    viewerResp, _ := api.GetViewer(ctx, client)
    viewerID := viewerResp.Viewer.UserDetailFields.Id
    input.AssigneeId = &viewerID
}

resp, err := api.CreateIssue(ctx, client, input)
```

**Example: `buildIssueUpdateInput()`**
```go
func buildIssueUpdateInput(cmd *cobra.Command) api.IssueUpdateInput {
    input := api.IssueUpdateInput{}

    if title, _ := cmd.Flags().GetString("title"); title != "" {
        input.Title = &title
    }
    if description, _ := cmd.Flags().GetString("description"); description != "" {
        input.Description = &description
    }
    if priority, _ := cmd.Flags().GetInt("priority"); priority >= -1 {
        p := float64(priority)
        input.Priority = &p
    }
    // State handling (if state flag provided, lookup stateId)

    return input
}
```

### Response Handling

Write operations return mutation responses with nested objects:

**Pattern:**
```go
resp, err := api.CreateIssue(ctx, client, input)
if err != nil {
    output.Error(fmt.Sprintf("Failed to create issue: %v", err), plaintext, jsonOut)
    os.Exit(1)
}

// Unwrap response
issue := resp.IssueCreate.Issue

// Access fields through fragments
identifier := issue.IssueDetailFields.Identifier
title := issue.IssueDetailFields.Title
```

**Response structures:**
- `CreateIssueResponse.IssueCreate.Issue`
- `UpdateIssueResponse.IssueUpdate.Issue`
- `CreateCommentResponse.CommentCreate.Comment`

### Key Differences from Phase 1

| Aspect | Phase 1 (Filters) | Phase 2 (Inputs) |
|--------|------------------|------------------|
| Purpose | Query filtering | Data mutations |
| Structs | Filter types (IssueFilter) | Input types (IssueCreateInput) |
| Fields | Nested comparators | Direct value pointers |
| Required fields | Rare | Common (TeamId, IssueId) |
| Type conversions | Minimal | int → float64 for priority |

### Type Conversions

**Priority: int → float64**
```go
priority, _ := cmd.Flags().GetInt("priority")  // User provides 0-4
priorityFloat := float64(priority)
input.Priority = &priorityFloat
```

**Strings: value → pointer**
```go
title, _ := cmd.Flags().GetString("title")
input.Title = &title  // Direct pointer to string
```

## Migration Tasks

### Task 1: Migrate `issue assign`

**Complexity:** Trivial
**Pattern:** Inline input building
**File:** `cmd/issue.go`

**Current implementation:**
```go
input := map[string]interface{}{
    "assigneeId": viewer.ID,
}
issue, err := client.UpdateIssue(context.Background(), args[0], input)
```

**New implementation:**
```go
viewerResp, err := api.GetViewer(ctx, client)
viewerID := viewerResp.Viewer.UserDetailFields.Id

input := api.IssueUpdateInput{
    AssigneeId: &viewerID,
}
resp, err := api.UpdateIssue(ctx, client, args[0], input)
issue := resp.IssueUpdate.Issue
```

**Changes:**
1. Replace `client.GetViewer()` with `api.GetViewer()`
2. Build typed `IssueUpdateInput` with `AssigneeId` field
3. Call `api.UpdateIssue()` instead of adapter
4. Unwrap response: `resp.IssueUpdate.Issue`
5. Update field access for output (if needed)

**Testing:**
- Assign issue to self
- Verify assignment in Linear
- Test invalid issue ID
- Test JSON output

### Task 2: Migrate `comment create`

**Complexity:** Simple
**Pattern:** Inline input building
**File:** `cmd/comment.go`

**Current implementation:**
```go
input := map[string]interface{}{
    "body":    body,
    "issueId": issueID,
}
comment, err := client.CreateComment(context.Background(), input)
```

**New implementation:**
```go
bodyPtr := body
input := api.CommentCreateInput{
    Body:    &bodyPtr,
    IssueId: issueID,
}
resp, err := api.CreateComment(ctx, client, input)
comment := resp.CommentCreate.Comment
```

**Changes:**
1. Build typed `CommentCreateInput` with `Body` and `IssueId`
2. Call `api.CreateComment()` instead of adapter
3. Unwrap response: `resp.CommentCreate.Comment`
4. Update field access for output

**Testing:**
- Create simple comment
- Create comment with multiline body
- Create comment with markdown
- Test invalid issue ID
- Test all output modes

### Task 3: Migrate `issue create`

**Complexity:** Moderate
**Pattern:** Builder function
**File:** `cmd/issue.go`

**Current implementation:**
```go
input := map[string]interface{}{
    "title":  title,
    "teamId": team.ID,
}
if description != "" {
    input["description"] = description
}
if priority >= 0 && priority <= 4 {
    input["priority"] = priority
}
if assignToMe {
    input["assigneeId"] = viewer.ID
}
issue, err := client.CreateIssue(context.Background(), input)
```

**New implementation:**
```go
func buildIssueCreateInput(cmd *cobra.Command, teamID string) api.IssueCreateInput {
    input := api.IssueCreateInput{TeamId: teamID}

    if title, _ := cmd.Flags().GetString("title"); title != "" {
        input.Title = &title
    }
    if description, _ := cmd.Flags().GetString("description"); description != "" {
        input.Description = &description
    }
    if priority, _ := cmd.Flags().GetInt("priority"); priority >= 0 && priority <= 4 {
        p := float64(priority)
        input.Priority = &p
    }

    return input
}

// In command handler
input := buildIssueCreateInput(cmd, team.ID)
if assignToMe {
    viewerResp, _ := api.GetViewer(ctx, client)
    viewerID := viewerResp.Viewer.UserDetailFields.Id
    input.AssigneeId = &viewerID
}

resp, err := api.CreateIssue(ctx, client, input)
issue := resp.IssueCreate.Issue
```

**Changes:**
1. Create `buildIssueCreateInput()` helper function
2. Handle required fields (TeamId) vs optional fields (pointers)
3. Convert priority: int → float64
4. Call `api.CreateIssue()` instead of adapter
5. Unwrap response: `resp.IssueCreate.Issue`
6. Update field access for output

**Testing:**
- Create minimal issue (title + team)
- Create with all fields
- Create with each priority level (0-4)
- Test missing required fields
- Test invalid team key
- Test all output modes

### Task 4: Migrate `issue update`

**Complexity:** Most complex
**Pattern:** Builder function
**File:** `cmd/issue.go`

**Current implementation:**
```go
input := map[string]interface{}{}
if title != "" {
    input["title"] = title
}
if state != "" {
    // Lookup stateId from state name
    input["stateId"] = stateID
}
if priority >= 0 {
    input["priority"] = priority
}
issue, err := client.UpdateIssue(context.Background(), issueID, input)
```

**New implementation:**
```go
func buildIssueUpdateInput(cmd *cobra.Command) api.IssueUpdateInput {
    input := api.IssueUpdateInput{}

    if title, _ := cmd.Flags().GetString("title"); title != "" {
        input.Title = &title
    }
    if description, _ := cmd.Flags().GetString("description"); description != "" {
        input.Description = &description
    }
    if priority, _ := cmd.Flags().GetInt("priority"); priority >= -1 {
        p := float64(priority)
        input.Priority = &p
    }

    return input
}

// In command handler
input := buildIssueUpdateInput(cmd)

// Handle state separately (requires lookup)
if state != "" {
    // Lookup stateId from state name
    input.StateId = &stateID
}

resp, err := api.UpdateIssue(ctx, client, issueID, input)
issue := resp.IssueUpdate.Issue
```

**Changes:**
1. Create `buildIssueUpdateInput()` helper function
2. All fields optional (use pointers)
3. Handle state name → stateId lookup (existing logic)
4. Convert priority: int → float64
5. Call `api.UpdateIssue()` instead of adapter
6. Unwrap response: `resp.IssueUpdate.Issue`
7. Update field access for output

**Testing:**
- Update title only
- Update state only
- Update priority only
- Update multiple fields together
- Test invalid issue ID
- Test invalid state name
- Test all output modes

## Testing Strategy

### Manual Testing with Documented Checklists

Each task includes a specific test checklist to be executed after migration. Tests use real Linear workspace data.

**Why manual testing:**
- Write operations shouldn't be automated against production workspace
- Real-world verification is most reliable
- Test checklists are reusable for regression testing
- Matches how commands are actually used

**Test documentation:**
- Each test case documented in task description
- Results recorded in completion report
- Any issues discovered documented and fixed

### Test Checklist Summary

**Total test cases: 16**
- Task 1 (issue assign): 4 test cases
- Task 2 (comment create): 5 test cases
- Task 3 (issue create): 6 test cases
- Task 4 (issue update): 7 test cases

All test cases cover:
- Happy path (successful operation)
- Error cases (invalid IDs, missing fields)
- Output modes (default, plaintext, JSON)
- Edge cases (multiline text, special characters)

## Error Handling

### Compile-Time Errors (Caught Early)

**Benefits of typed inputs:**
- Wrong field types → compile error
- Missing required fields → compile error (TeamId)
- Typos in field names → compile error
- Invalid enum values → compile error

**Example:**
```go
input := api.IssueCreateInput{
    TeamId: teamID,    // Required - must provide
    Title:  title,     // Compile error: need pointer
}
// Fix: Title: &title
```

### Runtime Errors (Still Need Handling)

**Types of runtime errors:**
- Invalid IDs (issue, team, state not found)
- Permission errors (can't modify issue)
- Network failures
- GraphQL errors from Linear API

**Pattern (unchanged from Phase 1):**
```go
resp, err := api.CreateIssue(ctx, client, input)
if err != nil {
    output.Error(fmt.Sprintf("Failed to create issue: %v", err), plaintext, jsonOut)
    os.Exit(1)
}
```

Error handling remains identical to Phase 1 - no changes needed.

## Success Criteria

Phase 2 completes when:

- ✅ All 4 write commands migrated to direct types
- ✅ All manual test checklists passed (16 test cases total)
- ✅ No regressions in functionality or output
- ✅ Build succeeds with no compilation errors
- ✅ Each command tested individually after migration
- ✅ Adapter functions marked deprecated (4 functions)
- ✅ Documentation updated (CLAUDE.md)
- ✅ Completion report created

## Expected Code Changes

### Files Modified

**Command files:**
- `cmd/issue.go` - 3 commands (assign, create, update)
- `cmd/comment.go` - 1 command (create)

**API files:**
- `pkg/api/adapter.go` - Mark 4 write functions deprecated:
  - `CreateIssue()` → Use `api.CreateIssue()` directly
  - `UpdateIssue()` → Use `api.UpdateIssue()` directly
  - `CreateComment()` → Use `api.CreateComment()` directly

**Documentation:**
- `CLAUDE.md` - Update Phase 2 status

### Lines Changed Estimate

- Command files: ~200-300 lines modified
- Builder functions added: ~100 lines (2 functions × 50 lines)
- Deprecation comments: ~8 lines
- Documentation: ~50 lines updated

**Total:** ~350-450 lines changed

## Risks and Mitigations

### Risk: Complex input structs with 30+ fields are hard to build correctly

**Mitigation:**
- Start with simplest command (issue assign - 1 field)
- Build complexity gradually
- Builder functions encapsulate complexity for complex commands
- Compiler catches missing required fields

**Likelihood:** Low
**Impact:** Medium

### Risk: Type conversions (int → float64) introduce bugs

**Mitigation:**
- Explicit conversions in code (visible to reviewers)
- Manual testing catches conversion issues
- Priority is small range (0-4), easy to verify

**Likelihood:** Low
**Impact:** Low

### Risk: Required vs optional field confusion

**Mitigation:**
- Generated types are explicit: `TeamId string` vs `AssigneeId *string`
- Compiler enforces required fields
- Phase 1 established patterns for pointer handling

**Likelihood:** Very Low
**Impact:** Low (caught at compile time)

### Risk: Manual testing misses edge cases

**Mitigation:**
- Documented test checklists ensure thoroughness
- Test all output modes
- Test error cases
- Real workspace testing is most reliable

**Likelihood:** Low
**Impact:** Medium

## Phase 3 Preview: Cleanup

After Phase 2 completes, Phase 3 will:

### Deletions
- Delete entire `pkg/api/adapter.go` (~1700 lines)
- Delete all conversion functions (convertToIssueCreateInput, etc.)
- Delete unused legacy types from `pkg/api/types.go`

### Documentation Updates
- Update CLAUDE.md to remove all adapter references
- Update "Adding New Commands" section for write operations
- Document Phase 3 completion

### Result
- **Single source of truth:** genqlient-generated types only
- **Zero maintenance burden:** No adapter layer to maintain
- **Full type safety:** All operations (read + write) use typed structs

### Total Code Reduction (All Phases)

**Phase 0:** -1,604 lines (queries.go + legacy.go)
**Phase 1:** -61 lines (buildIssueFilter)
**Phase 2:** ~0 lines (refactoring, not removal)
**Phase 3:** ~-1,700 lines (adapter.go + unused types)

**Total:** ~-3,365 lines removed across all phases

## Implementation Next Steps

1. Create design document (this file)
2. Set up git worktree for Phase 2 work
3. Create detailed implementation plan with task breakdowns
4. Execute tasks 1-4 incrementally with testing between
5. Document completion and merge to main
6. Plan Phase 3 (cleanup)

---

**Status:** Design complete, ready for implementation planning
