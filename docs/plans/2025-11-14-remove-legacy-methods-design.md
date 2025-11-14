# Remove Legacy Methods Design

**Date:** 2025-11-14
**Status:** Approved
**Author:** Architecture Review

## Problem Statement

The file `pkg/api/legacy.go` contains two unmigrated helper methods (`GetTeamStates` and `GetTeamMembers`) that use hand-written GraphQL queries instead of genqlient code generation. These methods were left behind during the main GraphQL migration and should be completed to finish the transition.

Additionally, `GetTeamStates` is inefficient - it makes an extra API call during every `issue update --state` command just to validate state names.

## Goals

1. **Complete the migration:** Remove all hand-written GraphQL code
2. **Optimize API calls:** Eliminate the extra `GetTeamStates` call by embedding states in issue fetches
3. **Maintain consistency:** All GraphQL operations use genqlient
4. **Preserve UX:** Keep user-friendly state names (no breaking changes)

## Current Usage

### GetTeamStates
- **Used in:** `cmd/issue.go` (issue update --state command)
- **Purpose:** Converts friendly state names ("In Progress") to state IDs
- **Problem:** Makes extra API call for every state update
- **Current flow:**
  1. Fetch issue to get team key
  2. Fetch team states to validate state name
  3. Update issue with state ID

### GetTeamMembers
- **Used in:** `cmd/team.go` (team members command)
- **Purpose:** Lists all members of a team
- **Problem:** Uses hand-written query instead of genqlient
- **Note:** GraphQL operation already defined in `teams.graphql` (line 51-69)

## Solution Design

### 1. Optimize GetTeamStates (Embed in Issue Fetch)

**Strategy:** Include team workflow states in the `GetIssue` query response. When fetching an issue, we automatically get its team's states.

**Implementation:**

Modify `pkg/api/operations/issues.graphql` - enhance `IssueDetailFields` fragment:

```graphql
fragment IssueDetailFields on Issue {
  # ... existing fields ...
  team {
    id
    name
    key
    # ADD: Workflow states
    states {
      nodes {
        id
        name
        type
        color
        description
        position
      }
    }
  }
  # ... rest of fields ...
}
```

**Result:**
- `GetIssue()` now returns `issue.Team.States[]`
- No separate API call needed
- State validation uses embedded data

**Command change in `cmd/issue.go`:**

```go
// Before: Two API calls
issue, err := client.GetIssue(ctx, issueID)
states, err := client.GetTeamStates(ctx, issue.Team.Key)  // EXTRA CALL

// After: One API call
issue, err := client.GetIssue(ctx, issueID)
// States already in issue.Team.States
for _, state := range issue.Team.States {
    if strings.EqualFold(state.Name, stateName) {
        stateID = state.ID
        break
    }
}
```

### 2. Migrate GetTeamMembers (Standard Migration)

**Strategy:** Complete the standard genqlient migration pattern.

**Implementation:**

1. GraphQL operation already exists in `operations/teams.graphql` (GetTeamMembers query)
2. Add adapter function in `pkg/api/adapter.go`:

```go
func (c *Client) GetTeamMembersNew(ctx context.Context, teamKey string) (*Users, error) {
    resp, err := GetTeamMembers(ctx, c, teamKey)
    if err != nil {
        return nil, err
    }
    return convertTeamMembersToLegacyUsers(resp), nil
}
```

3. Add conversion helper:

```go
func convertTeamMembersToLegacyUsers(resp *GetTeamMembersResponse) *Users {
    users := &Users{
        Nodes: make([]User, len(resp.Team.Members.Nodes)),
    }

    for i, member := range resp.Team.Members.Nodes {
        users.Nodes[i] = convertUserFieldsToLegacy(member)
    }

    if resp.Team.Members.PageInfo != nil {
        users.PageInfo.HasNextPage = resp.Team.Members.PageInfo.HasNextPage
        if resp.Team.Members.PageInfo.EndCursor != nil {
            users.PageInfo.EndCursor = *resp.Team.Members.PageInfo.EndCursor
        }
    }

    return users
}
```

4. Update `cmd/team.go`:

```go
// Before:
members, err := client.GetTeamMembers(context.Background(), teamKey)

// After:
members, err := client.GetTeamMembersNew(context.Background(), teamKey)
```

## Type Definitions

Ensure `WorkflowState` type exists in `pkg/api/types.go`:

```go
type WorkflowState struct {
    ID          string  `json:"id"`
    Name        string  `json:"name"`
    Type        string  `json:"type"`
    Color       string  `json:"color"`
    Description *string `json:"description"`
    Position    float64 `json:"position"`
}
```

This type is used for the team states embedded in issues and returned by the (now deleted) `GetTeamStates`.

## Files Modified

**Modified:**
- `pkg/api/operations/issues.graphql` - Add states to IssueDetailFields
- `pkg/api/adapter.go` - Add GetTeamMembersNew and conversion helpers
- `pkg/api/types.go` - Ensure WorkflowState type exists
- `cmd/issue.go` - Use embedded team.states instead of separate call
- `cmd/team.go` - Use GetTeamMembersNew

**Deleted:**
- `pkg/api/legacy.go` - 89 lines removed

**Regenerated:**
- `pkg/api/generated.go` - Includes new team states and team members operations

## Implementation Steps

1. Update `operations/issues.graphql` to include team states
2. Regenerate code: `cd pkg/api && go generate`
3. Add GetTeamMembersNew adapter and conversion helpers
4. Update `cmd/issue.go` to use embedded states
5. Update `cmd/team.go` to use GetTeamMembersNew
6. Delete `pkg/api/legacy.go`
7. Run tests: `make test`
8. Commit changes

## Benefits

**Performance:**
- Eliminates 1 API call per `issue update --state` command
- Reduces latency for state updates

**Code Quality:**
- Removes all hand-written GraphQL queries
- 100% genqlient code generation achieved
- Consistent architecture across all entities

**Maintainability:**
- No special-case legacy code to maintain
- Team states automatically update with schema changes
- All operations follow same pattern

## Testing

**Verify:**
- `lincli issue update LIN-123 --state "In Progress"` works (state validation)
- `lincli team members ENG` works (shows team members)
- All smoke tests pass (39/39)
- No regression in functionality

## Success Criteria

- [ ] `pkg/api/legacy.go` deleted
- [ ] Team states embedded in issue fetch
- [ ] GetTeamMembers uses genqlient adapter
- [ ] All commands work correctly
- [ ] All smoke tests pass
- [ ] No extra API calls for state validation

## Non-Goals

- Changing command-line interface
- Modifying output formats
- Adding new features
- Performance optimization beyond eliminating the extra call
